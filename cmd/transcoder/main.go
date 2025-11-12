// Command transcoder coordinates FFmpeg-based transcoding jobs.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type rendition struct {
	Name         string `json:"name"`
	ManifestURL  string `json:"manifestUrl"`
	Bitrate      int    `json:"bitrate,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	VideoBitrate int    `json:"videoBitrate,omitempty"`
	AudioBitrate int    `json:"audioBitrate,omitempty"`
	VideoProfile string `json:"videoProfile,omitempty"`
}

type job struct {
	ID         string
	ChannelID  string
	SessionID  string
	OriginURL  string
	Renditions []rendition
	OutputPath string
	Playback   string
	CreatedAt  time.Time
	StoppedAt  *time.Time
}

type uploadJob struct {
	ID          string
	ChannelID   string
	UploadID    string
	SourceURL   string
	Filename    string
	Renditions  []rendition
	OutputPath  string
	Playback    string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

type processState struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
}

type server struct {
	httpServer *http.Server
	token      string
	outputRoot string
	publicRoot string
	publicBase string
	mu         sync.RWMutex
	jobs       map[string]*job
	uploads    map[string]*uploadJob
	processes  map[string]*processState
	store      *metadataStore
}

type metadataStore struct {
	root string
}

type jobRequest struct {
	ChannelID  string          `json:"channelId"`
	SessionID  string          `json:"sessionId"`
	OriginURL  string          `json:"originUrl"`
	Renditions json.RawMessage `json:"renditions"`
}

type jobResponse struct {
	JobID      string          `json:"jobId"`
	JobIDs     []string        `json:"jobIds"`
	Renditions json.RawMessage `json:"renditions"`
}

type uploadRequest struct {
	ChannelID  string          `json:"channelId"`
	UploadID   string          `json:"uploadId"`
	SourceURL  string          `json:"sourceUrl"`
	Filename   string          `json:"filename"`
	Renditions json.RawMessage `json:"renditions"`
}

type uploadResponse struct {
	JobID       string          `json:"jobId"`
	PlaybackURL string          `json:"playbackUrl"`
	Renditions  json.RawMessage `json:"renditions"`
}

func main() {
	bind := envOrDefault("JOB_CONTROLLER_BIND", ":9000")
	token := strings.TrimSpace(os.Getenv("JOB_CONTROLLER_TOKEN"))
	if token == "" {
		log.Fatal("JOB_CONTROLLER_TOKEN must be configured before starting the transcoder")
	}
	outputRoot := envOrDefault("JOB_CONTROLLER_OUTPUT_ROOT", "./work")

	srv, err := newServer(token, outputRoot)
	if err != nil {
		log.Fatalf("initialise server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              bind,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	srv.httpServer = httpServer

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("ffmpeg job controller listening on %s", bind)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("ffmpeg job controller stopped")
}

func newServer(token, outputRoot string) (*server, error) {
	store, err := newMetadataStore(outputRoot)
	if err != nil {
		return nil, err
	}
	jobs, uploads, err := store.Load()
	if err != nil {
		return nil, err
	}
	publicBase := strings.TrimSpace(os.Getenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL"))
	if publicBase == "" {
		return nil, fmt.Errorf("BITRIVER_TRANSCODER_PUBLIC_BASE_URL must be configured before starting the transcoder")
	}
	mirrorRoot := strings.TrimSpace(os.Getenv("BITRIVER_TRANSCODER_PUBLIC_DIR"))
	if mirrorRoot == "" {
		mirrorRoot = filepath.Join(store.root, "public")
	}
	absMirror, err := filepath.Abs(mirrorRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve public mirror: %w", err)
	}
	if err := os.MkdirAll(absMirror, 0o755); err != nil {
		return nil, fmt.Errorf("prepare public mirror: %w", err)
	}
	for _, sub := range []string{"live", "uploads"} {
		if err := os.MkdirAll(filepath.Join(absMirror, sub), 0o755); err != nil {
			return nil, fmt.Errorf("prepare public mirror: %w", err)
		}
	}
	srv := &server{
		token:      token,
		outputRoot: store.root,
		publicBase: publicBase,
		publicRoot: absMirror,
		jobs:       jobs,
		uploads:    uploads,
		processes:  make(map[string]*processState),
		store:      store,
	}
	srv.restoreActiveProcesses()
	return srv, nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/v1/jobs", s.handleJobs)
	mux.HandleFunc("/v1/jobs/", s.handleJobByID)
	mux.HandleFunc("/v1/uploads", s.handleUploads)
	return logRequests(mux)
}

func (s *server) restoreActiveProcesses() {
	for id, jb := range s.jobs {
		if jb == nil {
			continue
		}
		if jb.StoppedAt != nil {
			if err := s.removeLiveMirror(id); err != nil {
				log.Printf("cleanup live mirror %s: %v", id, err)
			}
			continue
		}
		outputDir := jb.OutputPath
		if strings.TrimSpace(outputDir) == "" {
			outputDir = filepath.Join(s.outputRoot, "live", jb.ID)
		}
		plan, err := buildTranscodePlan(jb.OriginURL, outputDir, jb.Renditions)
		if err != nil {
			log.Printf("resume job %s: %v", id, err)
			continue
		}
		proc, err := s.startFFmpeg(id, plan, s.makeJobExitHandler(id))
		if err != nil {
			log.Printf("restart job %s: %v", id, err)
			continue
		}
		jb.Renditions = cloneRenditions(plan.renditions)
		jb.OutputPath = plan.outputDir
		jb.Playback = plan.master
		s.processes[id] = proc
		if err := s.store.SaveJob(jb); err != nil {
			log.Printf("persist job %s: %v", id, err)
		}
		if err := s.publishLive(jb); err != nil {
			log.Printf("publish live job %s: %v", id, err)
		}
	}
	for id, up := range s.uploads {
		if up == nil || up.CompletedAt != nil {
			continue
		}
		outputDir := up.OutputPath
		if strings.TrimSpace(outputDir) == "" {
			outputDir = filepath.Join(s.outputRoot, "uploads", up.ID)
		}
		plan, err := buildTranscodePlan(up.SourceURL, outputDir, up.Renditions)
		if err != nil {
			log.Printf("resume upload %s: %v", id, err)
			continue
		}
		proc, err := s.startFFmpeg(id, plan, s.makeUploadExitHandler(id))
		if err != nil {
			log.Printf("restart upload %s: %v", id, err)
			continue
		}
		up.Renditions = cloneRenditions(plan.renditions)
		up.OutputPath = plan.outputDir
		up.Playback = plan.master
		s.processes[id] = proc
		if err := s.store.SaveUpload(up); err != nil {
			log.Printf("persist upload %s: %v", id, err)
		}
	}
}

func (s *server) authorize(r *http.Request) bool {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return false
	}
	token := strings.TrimSpace(header[7:])
	return token == s.token
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req jobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ChannelID) == "" || strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.OriginURL) == "" {
		http.Error(w, "channelId, sessionId, and originUrl are required", http.StatusBadRequest)
		return
	}
	renditions, err := decodeRenditions(req.Renditions)
	if err != nil {
		http.Error(w, "invalid renditions", http.StatusBadRequest)
		return
	}

	jobID := newID("live")
	plan, err := buildTranscodePlan(req.OriginURL, filepath.Join(s.outputRoot, "live", jobID), renditions)
	if err != nil {
		http.Error(w, "unable to prepare transcode", http.StatusInternalServerError)
		return
	}

	meta := &job{
		ID:         jobID,
		ChannelID:  req.ChannelID,
		SessionID:  req.SessionID,
		OriginURL:  req.OriginURL,
		Renditions: cloneRenditions(plan.renditions),
		OutputPath: plan.outputDir,
		Playback:   plan.master,
		CreatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.jobs[jobID] = meta
	s.mu.Unlock()

	proc, err := s.startFFmpeg(jobID, plan, s.makeJobExitHandler(jobID))
	if err != nil {
		s.mu.Lock()
		delete(s.jobs, jobID)
		s.mu.Unlock()
		http.Error(w, "failed to start ffmpeg", http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.processes[jobID] = proc
	s.mu.Unlock()

	if err := s.store.SaveJob(meta); err != nil {
		s.mu.Lock()
		delete(s.jobs, jobID)
		delete(s.processes, jobID)
		s.mu.Unlock()
		proc.cancel()
		<-proc.done
		http.Error(w, "failed to persist job", http.StatusInternalServerError)
		return
	}

	if err := s.publishLive(meta); err != nil {
		log.Printf("publish live job %s: %v", jobID, err)
	}

	publicRenditions := cloneRenditions(plan.renditions)
	for i := range publicRenditions {
		rel := relativeLocation(plan.outputDir, publicRenditions[i].ManifestURL)
		if rel == "" {
			rel = filepath.ToSlash(filepath.Base(filepath.FromSlash(publicRenditions[i].ManifestURL)))
		}
		publicRenditions[i].ManifestURL = s.publicLiveURL(jobID, rel)
	}

	resp := jobResponse{
		JobID:      jobID,
		JobIDs:     []string{jobID},
		Renditions: encodeRenditions(publicRenditions),
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *server) handleJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	meta, ok := s.jobs[id]
	proc := s.processes[id]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	if proc != nil {
		proc.cancel()
		select {
		case <-proc.done:
		case <-time.After(15 * time.Second):
			log.Printf("timeout waiting for job %s to stop", id)
		}
	}

	now := time.Now().UTC()
	meta.StoppedAt = &now
	if err := s.store.SaveJob(meta); err != nil {
		log.Printf("persist stopped job %s: %v", id, err)
	}
	if err := s.removeLiveMirror(id); err != nil {
		log.Printf("cleanup live mirror %s: %v", id, err)
	}

	s.mu.Lock()
	delete(s.jobs, id)
	delete(s.processes, id)
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleUploads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req uploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ChannelID) == "" || strings.TrimSpace(req.UploadID) == "" || strings.TrimSpace(req.SourceURL) == "" {
		http.Error(w, "channelId, uploadId, and sourceUrl are required", http.StatusBadRequest)
		return
	}
	renditions, err := decodeRenditions(req.Renditions)
	if err != nil {
		http.Error(w, "invalid renditions", http.StatusBadRequest)
		return
	}

	jobID := newID("upload")
	plan, err := buildTranscodePlan(req.SourceURL, filepath.Join(s.outputRoot, "uploads", jobID), renditions)
	if err != nil {
		http.Error(w, "unable to prepare transcode", http.StatusInternalServerError)
		return
	}

	meta := &uploadJob{
		ID:         jobID,
		ChannelID:  req.ChannelID,
		UploadID:   req.UploadID,
		SourceURL:  req.SourceURL,
		Filename:   req.Filename,
		Renditions: cloneRenditions(plan.renditions),
		OutputPath: plan.outputDir,
		Playback:   plan.master,
		CreatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.uploads[jobID] = meta
	s.mu.Unlock()

	proc, err := s.startFFmpeg(jobID, plan, s.makeUploadExitHandler(jobID))
	if err != nil {
		s.mu.Lock()
		delete(s.uploads, jobID)
		s.mu.Unlock()
		http.Error(w, "failed to start ffmpeg", http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.processes[jobID] = proc
	s.mu.Unlock()

	if err := s.store.SaveUpload(meta); err != nil {
		s.mu.Lock()
		delete(s.uploads, jobID)
		delete(s.processes, jobID)
		s.mu.Unlock()
		proc.cancel()
		<-proc.done
		http.Error(w, "failed to persist upload", http.StatusInternalServerError)
		return
	}

	publicRenditions := cloneRenditions(plan.renditions)
	playback := meta.Playback
	if s.publicBase != "" {
		masterRel := relativeLocation(plan.outputDir, plan.master)
		if masterRel == "" {
			masterRel = "index.m3u8"
		}
		playback = s.publicUploadURL(jobID, masterRel)
		for i := range publicRenditions {
			rel := relativeLocation(plan.outputDir, publicRenditions[i].ManifestURL)
			if rel == "" {
				rel = filepath.ToSlash(filepath.Base(filepath.FromSlash(publicRenditions[i].ManifestURL)))
			}
			publicRenditions[i].ManifestURL = s.publicUploadURL(jobID, rel)
		}
	}
	resp := uploadResponse{
		JobID:       jobID,
		PlaybackURL: playback,
		Renditions:  encodeRenditions(publicRenditions),
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (s *server) makeJobExitHandler(id string) func(error) {
	return func(err error) {
		now := time.Now().UTC()
		var meta *job
		s.mu.Lock()
		if j, ok := s.jobs[id]; ok {
			if j.StoppedAt == nil {
				j.StoppedAt = &now
			}
			meta = j
			delete(s.jobs, id)
		}
		delete(s.processes, id)
		s.mu.Unlock()
		if meta != nil {
			if saveErr := s.store.SaveJob(meta); saveErr != nil {
				log.Printf("persist job %s: %v", id, saveErr)
			}
		}
		if err := s.removeLiveMirror(id); err != nil {
			log.Printf("cleanup live mirror %s: %v", id, err)
		}
	}
}

func (s *server) makeUploadExitHandler(id string) func(error) {
	return func(err error) {
		now := time.Now().UTC()
		var meta *uploadJob
		var publish bool
		s.mu.Lock()
		if up, ok := s.uploads[id]; ok {
			if up.CompletedAt == nil {
				up.CompletedAt = &now
			}
			meta = up
			publish = err == nil
		}
		delete(s.processes, id)
		s.mu.Unlock()
		if publish && meta != nil {
			if err := s.publishUpload(meta); err != nil {
				log.Printf("publish upload %s: %v", id, err)
			}
		}
		if meta != nil {
			if saveErr := s.store.SaveUpload(meta); saveErr != nil {
				log.Printf("persist upload %s: %v", id, saveErr)
			}
		}
	}
}

func decodeRenditions(raw json.RawMessage) ([]rendition, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var payload []rendition
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]rendition, len(payload))
	copy(out, payload)
	return out, nil
}

func encodeRenditions(r []rendition) json.RawMessage {
	if len(r) == 0 {
		return json.RawMessage("[]")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return json.RawMessage("[]")
	}
	return data
}

func cloneRenditions(src []rendition) []rendition {
	if len(src) == 0 {
		return nil
	}
	out := make([]rendition, len(src))
	copy(out, src)
	return out
}

type transcodePlan struct {
	args       []string
	renditions []rendition
	outputDir  string
	master     string
}

func buildTranscodePlan(input, outputDir string, ladder []rendition) (*transcodePlan, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("input source is required")
	}
	if strings.TrimSpace(outputDir) == "" {
		return nil, fmt.Errorf("output directory is required")
	}
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, err
	}

	updated := make([]rendition, len(ladder))
	copy(updated, ladder)
	if len(updated) == 0 {
		updated = append(updated, rendition{Name: "720p", Bitrate: 2800})
	}

	count := len(updated)
	master := filepath.ToSlash(filepath.Join(absDir, "index.m3u8"))
	variantNames := make([]string, count)
	videoBitrates := make([]int, count)
	audioBitrates := make([]int, count)
	widths := make([]int, count)
	heights := make([]int, count)
	profiles := make([]string, count)

	filters := make([]string, 0, count+1)
	splitLabels := make([]string, count)
	if count > 1 {
		for idx := range updated {
			splitLabels[idx] = fmt.Sprintf("splitv%d", idx)
		}
		filters = append(filters, fmt.Sprintf("[0:v]split=%d[%s]", count, strings.Join(splitLabels, "][")))
	}

	used := make(map[string]int)
	for idx := range updated {
		base := sanitizeName(updated[idx].Name)
		if base == "" {
			base = fmt.Sprintf("variant-%d", idx)
		}
		countForBase := used[base]
		name := base
		if countForBase > 0 {
			name = fmt.Sprintf("%s-%d", base, countForBase)
		}
		used[base] = countForBase + 1
		if err := os.MkdirAll(filepath.Join(absDir, name), 0o755); err != nil {
			return nil, err
		}

		width, height := resolveDimensions(updated[idx].Name)
		widths[idx] = width
		heights[idx] = height
		profile := videoProfileForHeight(height)
		profiles[idx] = profile

		videoTarget := updated[idx].Bitrate
		if videoTarget <= 0 {
			videoTarget = defaultVideoBitrate(height)
		}
		audioTarget := defaultAudioBitrate(videoTarget)
		totalBitrate := videoTarget + audioTarget

		updated[idx].Width = width
		updated[idx].Height = height
		updated[idx].VideoBitrate = videoTarget
		updated[idx].AudioBitrate = audioTarget
		updated[idx].VideoProfile = profile
		updated[idx].Bitrate = totalBitrate
		updated[idx].ManifestURL = filepath.ToSlash(filepath.Join(absDir, name, "index.m3u8"))

		inputLabel := "[0:v]"
		if count > 1 {
			inputLabel = fmt.Sprintf("[%s]", splitLabels[idx])
		}
		filters = append(filters, fmt.Sprintf("%s%s[v%d]", inputLabel, buildScaleFilter(width, height), idx))

		variantNames[idx] = name
		videoBitrates[idx] = videoTarget
		audioBitrates[idx] = audioTarget
	}

	args := []string{
		"-y",
		"-i", input,
	}
	if len(filters) > 0 {
		args = append(args, "-filter_complex", strings.Join(filters, ";"))
	}

	for idx := range updated {
		args = append(args, "-map", fmt.Sprintf("[v%d]", idx))
		args = append(args, "-map", "0:a:0?")
	}

	args = append(args, "-preset", "veryfast", "-pix_fmt", "yuv420p")

	for idx := range updated {
		stream := strconv.Itoa(idx)
		videoTarget := videoBitrates[idx]
		audioTarget := audioBitrates[idx]
		maxRate := int(math.Round(float64(videoTarget) * 1.08))
		if maxRate <= videoTarget {
			maxRate = videoTarget + 1
		}
		args = append(args,
			"-c:v:"+stream, "libx264",
			"-profile:v:"+stream, profiles[idx],
			"-b:v:"+stream, fmt.Sprintf("%dk", videoTarget),
			"-maxrate:v:"+stream, fmt.Sprintf("%dk", maxRate),
			"-bufsize:v:"+stream, fmt.Sprintf("%dk", videoTarget*2),
			"-g:v:"+stream, "48",
			"-keyint_min:v:"+stream, "48",
			"-sc_threshold:v:"+stream, "0",
		)
		args = append(args,
			"-c:a:"+stream, "aac",
			"-b:a:"+stream, fmt.Sprintf("%dk", audioTarget),
			"-ac:a:"+stream, "2",
			"-ar:a:"+stream, "48000",
		)
	}

	segmentPattern := filepath.ToSlash(filepath.Join(absDir, "%v", "segment_%06d.ts"))
	varStreamMap := make([]string, 0, len(updated))
	for idx := range updated {
		bandwidth := (videoBitrates[idx] + audioBitrates[idx]) * 1000
		entry := fmt.Sprintf("v:%d,a:%d name:%s bandwidth:%d resolution:%dx%d", idx, idx, variantNames[idx], bandwidth, widths[idx], heights[idx])
		varStreamMap = append(varStreamMap, entry)
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "4",
		"-hls_list_size", "6",
		"-hls_flags", "delete_segments+program_date_time+independent_segments",
		"-master_pl_name", "index.m3u8",
		"-hls_segment_filename", segmentPattern,
		"-var_stream_map", strings.Join(varStreamMap, " "),
		filepath.ToSlash(filepath.Join(absDir, "%v", "index.m3u8")),
	)

	return &transcodePlan{
		args:       args,
		renditions: updated,
		outputDir:  absDir,
		master:     master,
	}, nil
}

func (s *server) startFFmpeg(jobID string, plan *transcodePlan, onExit func(error)) (*processState, error) {
	if plan == nil {
		return nil, fmt.Errorf("transcode plan is required")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "ffmpeg", plan.args...)
	cmd.Stdout = newLogWriter(jobID, "stdout")
	cmd.Stderr = newLogWriter(jobID, "stderr")
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	proc := &processState{cmd: cmd, cancel: cancel, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("ffmpeg %s exited with error: %v", jobID, err)
		} else {
			log.Printf("ffmpeg %s completed", jobID)
		}
		if onExit != nil {
			onExit(err)
		}
		cancel()
		close(proc.done)
	}()
	return proc, nil
}

type logWriter struct {
	prefix string
}

func newLogWriter(jobID, stream string) *logWriter {
	return &logWriter{prefix: fmt.Sprintf("[%s][%s] ", jobID, stream)}
}

func (w *logWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		var line []byte
		if idx == -1 {
			line = p
			p = nil
		} else {
			line = p[:idx]
			p = p[idx+1:]
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		log.Printf("%s%s", w.prefix, string(line))
	}
	return total, nil
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "variant"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "variant"
	}
	return b.String()
}

var resolutionPattern = regexp.MustCompile(`(?i)(\d{3,4})p`)

func resolveDimensions(name string) (int, int) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return 1280, 720
	}
	parts := strings.Split(trimmed, "x")
	if len(parts) == 2 {
		if w, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && w > 0 {
			if h, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && h > 0 {
				return ensureEven(w), ensureEven(h)
			}
		}
	}
	if match := resolutionPattern.FindStringSubmatch(trimmed); len(match) == 2 {
		if h, err := strconv.Atoi(match[1]); err == nil && h > 0 {
			height := ensureEven(h)
			width := ensureEven(int(math.Round(float64(height) * 16.0 / 9.0)))
			if width <= 0 {
				width = 1280
			}
			return width, height
		}
	}
	return 1280, 720
}

func buildScaleFilter(width, height int) string {
	if width <= 0 || height <= 0 {
		width = 1280
		height = 720
	}
	width = ensureEven(width)
	height = ensureEven(height)
	return fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease,setsar=1,pad=%d:%d:(ow-iw)/2:(oh-ih)/2", width, height, width, height)
}

func ensureEven(value int) int {
	if value%2 != 0 {
		return value + 1
	}
	if value <= 0 {
		return 2
	}
	return value
}

func defaultVideoBitrate(height int) int {
	switch {
	case height >= 1080:
		return 6000
	case height >= 720:
		return 4000
	case height >= 540:
		return 3000
	case height >= 480:
		return 2200
	case height >= 360:
		return 1200
	case height >= 240:
		return 700
	default:
		return 500
	}
}

func defaultAudioBitrate(videoBitrate int) int {
	switch {
	case videoBitrate >= 5000:
		return 192
	case videoBitrate >= 3000:
		return 160
	case videoBitrate >= 1500:
		return 128
	case videoBitrate >= 800:
		return 96
	case videoBitrate > 0:
		return 64
	default:
		return 0
	}
}

func videoProfileForHeight(height int) string {
	switch {
	case height >= 1080:
		return "high"
	case height >= 720:
		return "high"
	case height >= 480:
		return "main"
	default:
		return "baseline"
	}
}

func newMetadataStore(root string) (*metadataStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("output root is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"live", "uploads"} {
		if err := os.MkdirAll(filepath.Join(absRoot, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &metadataStore{root: absRoot}, nil
}

func (m *metadataStore) Load() (map[string]*job, map[string]*uploadJob, error) {
	jobs := make(map[string]*job)
	uploads := make(map[string]*uploadJob)
	if err := loadJobMetadata(filepath.Join(m.root, "live"), jobs); err != nil {
		return nil, nil, err
	}
	if err := loadUploadMetadata(filepath.Join(m.root, "uploads"), uploads); err != nil {
		return nil, nil, err
	}
	return jobs, uploads, nil
}

func (m *metadataStore) SaveJob(j *job) error {
	if j == nil {
		return nil
	}
	dir := filepath.Join(m.root, "live", j.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if j.OutputPath == "" {
		j.OutputPath = dir
	}
	return writeJSONFile(filepath.Join(dir, "metadata.json"), j)
}

func (m *metadataStore) SaveUpload(u *uploadJob) error {
	if u == nil {
		return nil
	}
	dir := filepath.Join(m.root, "uploads", u.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if u.OutputPath == "" {
		u.OutputPath = dir
	}
	return writeJSONFile(filepath.Join(dir, "metadata.json"), u)
}

func loadJobMetadata(root string, dest map[string]*job) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(root, entry.Name(), "metadata.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read job metadata %s: %w", metaPath, err)
		}
		var j job
		if err := json.Unmarshal(data, &j); err != nil {
			return fmt.Errorf("decode job metadata %s: %w", metaPath, err)
		}
		if j.ID == "" {
			j.ID = entry.Name()
		}
		if j.OutputPath == "" {
			j.OutputPath = filepath.Join(root, entry.Name())
		}
		if j.Playback == "" {
			j.Playback = filepath.ToSlash(filepath.Join(j.OutputPath, "index.m3u8"))
		}
		dest[j.ID] = &j
	}
	return nil
}

func loadUploadMetadata(root string, dest map[string]*uploadJob) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(root, entry.Name(), "metadata.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read upload metadata %s: %w", metaPath, err)
		}
		var u uploadJob
		if err := json.Unmarshal(data, &u); err != nil {
			return fmt.Errorf("decode upload metadata %s: %w", metaPath, err)
		}
		if u.ID == "" {
			u.ID = entry.Name()
		}
		if u.OutputPath == "" {
			u.OutputPath = filepath.Join(root, entry.Name())
		}
		if u.Playback == "" {
			u.Playback = filepath.ToSlash(filepath.Join(u.OutputPath, "index.m3u8"))
		}
		dest[u.ID] = &u
	}
	return nil
}

func (s *server) publishLive(j *job) error {
	if s.publicBase == "" || j == nil {
		return nil
	}
	if strings.TrimSpace(j.ID) == "" {
		return fmt.Errorf("job id missing")
	}
	src := strings.TrimSpace(j.OutputPath)
	if src == "" {
		return fmt.Errorf("output path missing")
	}
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("resolve live output: %w", err)
	}
	dest := filepath.Join(s.publicRoot, "live", j.ID)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("prepare live mirror: %w", err)
	}
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			existing, linkErr := os.Readlink(dest)
			if linkErr == nil {
				if absExisting, absErr := filepath.Abs(existing); absErr == nil && absExisting == absSrc {
					return nil
				}
			}
		}
		if removeErr := os.Remove(dest); removeErr != nil {
			if !errors.Is(removeErr, os.ErrNotExist) {
				if removeAllErr := os.RemoveAll(dest); removeAllErr != nil {
					return fmt.Errorf("clear existing live mirror: %w", removeAllErr)
				}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat live mirror: %w", err)
	}
	if err := os.Symlink(absSrc, dest); err != nil {
		return fmt.Errorf("create live mirror: %w", err)
	}
	return nil
}

func (s *server) removeLiveMirror(jobID string) error {
	if s.publicBase == "" || strings.TrimSpace(jobID) == "" {
		return nil
	}
	dest := filepath.Join(s.publicRoot, "live", jobID)
	if err := os.Remove(dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if removeAllErr := os.RemoveAll(dest); removeAllErr != nil {
			return fmt.Errorf("remove live mirror: %w", removeAllErr)
		}
	}
	return nil
}

func (s *server) publishUpload(up *uploadJob) error {
	if s.publicBase == "" || up == nil {
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(up.Playback), s.publicBase) {
		return nil
	}
	src := strings.TrimSpace(up.OutputPath)
	if src == "" {
		return fmt.Errorf("output path missing")
	}
	dest := filepath.Join(s.publicRoot, "uploads", up.ID)
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("clear publish target: %w", err)
	}
	if err := copyDirectory(src, dest); err != nil {
		return fmt.Errorf("mirror upload: %w", err)
	}
	relMaster := "index.m3u8"
	if local := strings.TrimSpace(up.Playback); local != "" {
		if rel, err := filepath.Rel(src, filepath.FromSlash(local)); err == nil && rel != "." {
			relMaster = filepath.ToSlash(rel)
		}
	}
	up.Playback = s.publicUploadURL(up.ID, relMaster)
	for i := range up.Renditions {
		local := filepath.FromSlash(up.Renditions[i].ManifestURL)
		rel, err := filepath.Rel(src, local)
		if err != nil {
			rel = filepath.Base(local)
		}
		up.Renditions[i].ManifestURL = s.publicUploadURL(up.ID, filepath.ToSlash(rel))
	}
	return nil
}

func (s *server) publicLiveURL(jobID, rel string) string {
	if s.publicBase == "" {
		return ""
	}
	return joinURL(s.publicBase, "live", jobID, rel)
}

func (s *server) publicUploadURL(jobID, rel string) string {
	if s.publicBase == "" {
		return ""
	}
	return joinURL(s.publicBase, "uploads", jobID, rel)
}

func relativeLocation(baseDir, target string) string {
	base := strings.TrimSpace(baseDir)
	trimmed := strings.TrimSpace(target)
	if base == "" || trimmed == "" {
		return ""
	}
	converted := filepath.FromSlash(trimmed)
	rel, err := filepath.Rel(base, converted)
	if err != nil {
		return filepath.ToSlash(filepath.Base(converted))
	}
	if rel == "." {
		return filepath.ToSlash(filepath.Base(converted))
	}
	return filepath.ToSlash(rel)
}

func joinURL(base string, parts ...string) string {
	trimmed := strings.TrimRight(base, "/")
	addition := path.Join(parts...)
	if addition == "." {
		addition = ""
	}
	if addition == "" {
		return trimmed
	}
	if trimmed == "" {
		return "/" + strings.TrimLeft(addition, "/")
	}
	return trimmed + "/" + strings.TrimLeft(addition, "/")
}

func copyDirectory(src, dst string) error {
	return filepath.WalkDir(src, func(current string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, current)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			mode := fs.FileMode(0o755)
			if info, infoErr := d.Info(); infoErr == nil {
				mode = info.Mode()
			}
			return os.MkdirAll(target, mode.Perm())
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlinks not supported: %s", current)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(current)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			in.Close()
			return err
		}
		if err := out.Close(); err != nil {
			in.Close()
			return err
		}
		return in.Close()
	})
}

func writeJSONFile(path string, payload any) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "meta-*.tmp")
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, lrw.status, time.Since(start))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, rand.Int63())
}

func envOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
