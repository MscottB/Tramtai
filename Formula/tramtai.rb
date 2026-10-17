# Generado por scripts/paquetes.ps1 al publicar v0.41.0+17102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.41.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.41.0%2B17102026/tramtai-v0.41.0%2B17102026-macos-arm64.tar.gz"
      sha256 "c9a67306905a675d76a090991f4e92f14ef5d8bbd9c3660094060ab03d531993"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.41.0%2B17102026/tramtai-v0.41.0%2B17102026-macos-amd64.tar.gz"
      sha256 "d8c7e07f99a0769bfa543f29f5179405ffde0c34edd1f3bd96f46a25c5b6eef6"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.41.0%2B17102026/tramtai-v0.41.0%2B17102026-linux-arm64.tar.gz"
      sha256 "13128783179eea50cd64f68776e3ad7524f3124556f5ed5d56d8d25ab17d08dc"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.41.0%2B17102026/tramtai-v0.41.0%2B17102026-linux-amd64.tar.gz"
      sha256 "f1a1b4709b454dce8c1ab37a34f1105e66c0ae2272599b821d299e2b42c6d38c"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
