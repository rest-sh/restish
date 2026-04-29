# typed: strict
# frozen_string_literal: true

# Formula for the legacy Restish v1 binary.
class RestishAT1 < Formula
  desc "CLI for interacting with REST-ish HTTP APIs"
  homepage "https://rest.sh/"
  version "0.21.2"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/rest-sh/restish/releases/download/v0.21.2/restish-0.21.2-darwin-arm64.tar.gz"
    sha256 "99fa714dc7abd99afaa5f9d1b24cd8e2767c6b9a275de1b4cbfb0f246173af29"
  elsif OS.mac?
    url "https://github.com/rest-sh/restish/releases/download/v0.21.2/restish-0.21.2-darwin-amd64.tar.gz"
    sha256 "27d3423c0942348f8d8588d10a7a3fd2207627876082737fad53ec80e338dec4"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/rest-sh/restish/releases/download/v0.21.2/restish-0.21.2-linux-arm64.tar.gz"
    sha256 "80a26c35462c976fda2c318ba992db7ed57407c2dfc6b1124d43a5adfa06db7a"
  elsif OS.linux?
    url "https://github.com/rest-sh/restish/releases/download/v0.21.2/restish-0.21.2-linux-amd64.tar.gz"
    sha256 "00810b4c5def5c8bf92c9aec8ca796bb6615ca6d781772a7c3e37e2e2b658d46"
  end

  keg_only :versioned_formula

  def install
    bin.install "restish"
  end

  test do
    system "#{bin}/restish", "--version"
  end
end
