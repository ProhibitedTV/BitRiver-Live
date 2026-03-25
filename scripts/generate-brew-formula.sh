#!/bin/sh
set -eu

version="$1"
arm_url="$2"
arm_sha="$3"
amd_url="$4"
amd_sha="$5"
output_path="$6"

cat >"$output_path" <<FORMULA
class BitriverLive < Formula
  desc "Launcher for BitRiver Live"
  homepage "https://github.com/ProhibitedTV/BitRiver-Live"
  version "$version"

  on_macos do
    if Hardware::CPU.arm?
      url "$arm_url"
      sha256 "$arm_sha"
    else
      url "$amd_url"
      sha256 "$amd_sha"
    end
  end

  def install
    bin.install "bin/bitriver"
    bin.install "bin/bitriver-live"
    share.install Dir["share/*"]
  end

  def caveats
    <<~EOS
      BitRiver Live launcher installs Docker Compose assets under \n#{share}\nUse bitriver-live to pull images and start the stack.
    EOS
  end
end
FORMULA
