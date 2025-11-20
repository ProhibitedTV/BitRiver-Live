# Wrapper image that guarantees curl is available for health checks.
ARG SRS_VERSION=v5.0.185
FROM ossrs/srs:${SRS_VERSION}

RUN set -eu; \
    if command -v apt-get >/dev/null 2>&1; then \
      apt-get update && \
      apt-get install -y --no-install-recommends curl ca-certificates && \
      rm -rf /var/lib/apt/lists/*; \
    elif command -v apk >/dev/null 2>&1; then \
      apk add --no-cache curl; \
    elif command -v yum >/dev/null 2>&1; then \
      yum install -y curl ca-certificates && \
      yum clean all && \
      rm -rf /var/cache/yum; \
    else \
      echo "No supported package manager found to install curl" >&2; \
      exit 1; \
    fi
