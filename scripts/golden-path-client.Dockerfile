# syntax=docker/dockerfile:1

FROM python:3.12-slim

RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /harness
COPY scripts/production_golden_path.py ./production_golden_path.py
COPY scripts/capacity_qualification.py ./capacity_qualification.py
COPY scripts/capacity-scenarios ./capacity-scenarios

ENTRYPOINT ["python3", "/harness/production_golden_path.py"]
