# Generado por scripts/paquetes.ps1 al publicar v0.50.2+20102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.50.2"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.2%2B20102026/tramtai-v0.50.2%2B20102026-macos-arm64.tar.gz"
      sha256 "a82a6aebb91d22a879f11112e720d7c2fbc694bfde6cfe3db6a1a6340338c6ee"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.2%2B20102026/tramtai-v0.50.2%2B20102026-macos-amd64.tar.gz"
      sha256 "031dce5a18ec4fe92c4e9863e0cfd1250483ab16a7a34756ab1dc2790921387e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.2%2B20102026/tramtai-v0.50.2%2B20102026-linux-arm64.tar.gz"
      sha256 "373cc120901f62badc2ca559a9f4c8dfc634bac6bf24dac0eded711b6b3c0d29"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.2%2B20102026/tramtai-v0.50.2%2B20102026-linux-amd64.tar.gz"
      sha256 "bfee532ccb949af2a4245e6a6132ce642fbef6751d32c60bd15eb3705615fdb9"
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
