# Generado por scripts/paquetes.ps1 al publicar v0.52.0+20102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.52.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.52.0%2B20102026/tramtai-v0.52.0%2B20102026-macos-arm64.tar.gz"
      sha256 "f3efa587330aa64a1ce342de6fa26c9bd943e0ca01f1c661d273af098c651f8d"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.52.0%2B20102026/tramtai-v0.52.0%2B20102026-macos-amd64.tar.gz"
      sha256 "ce2686b03bcad1f2f90b3ee866d9593f2d1382c7efe2d36bc56f324588aff5ef"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.52.0%2B20102026/tramtai-v0.52.0%2B20102026-linux-arm64.tar.gz"
      sha256 "3be9eace7d81b7d34c6d439d03b5c57b85112836221e0bdb863042015e1d8591"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.52.0%2B20102026/tramtai-v0.52.0%2B20102026-linux-amd64.tar.gz"
      sha256 "9ead7f5d90c237b169a4a0cf161d64623c56364cc0baaa5bc8936f26e9dd34eb"
    end
  end

  def install
    bin.install "tramtai"
    (lib/"tramtai").install "uv" # prepara el conversor de archivos; Tramtai lo busca en ../lib/tramtai
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
