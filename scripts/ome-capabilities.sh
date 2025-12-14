#!/usr/bin/env bash
set -euo pipefail

compute_ome_capabilities() {
  local image_tag=$1
  local default_modern=${2:-1}

  local supports_access_token=$default_modern
  local supports_managers_authentication=$default_modern
  local supports_output_streams=$default_modern
  local supports_application_outputs=$default_modern
  local parsing_failed=0

  if [[ $image_tag =~ ^v?([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
    local major="${BASH_REMATCH[1]}"
    local minor="${BASH_REMATCH[2]}"
    if (( major == 0 && minor < 16 )); then
      supports_access_token=0
      supports_managers_authentication=0
      supports_output_streams=0
      supports_application_outputs=0
    fi
  else
    parsing_failed=1
  fi

  cat <<CAPS
supports_access_token=$supports_access_token
supports_managers_authentication=$supports_managers_authentication
supports_output_streams=$supports_output_streams
supports_application_outputs=$supports_application_outputs
parsing_failed=$parsing_failed
CAPS
}
