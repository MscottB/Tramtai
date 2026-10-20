# Generado por scripts/paquetes.ps1 al publicar v0.50.1+19102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.50.1"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.1%2B19102026/tramtai-v0.50.1%2B19102026-macos-arm64.tar.gz"
      sha256 "a4f2f8cef826b6863511cf74ae837f90c77bd180b0ff9ec96cc9e25050c9d1df"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.1%2B19102026/tramtai-v0.50.1%2B19102026-macos-amd64.tar.gz"
      sha256 "cba61e3b4d0a57cb62f36007bde66c9bc201dc43c687d7174f4ae25cffe49a57"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.1%2B19102026/tramtai-v0.50.1%2B19102026-linux-arm64.tar.gz"
      sha256 "73699b33a535fa16e9c0cd6776de5f86088a9336a19e3779de4b81f4940e0b49"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.1%2B19102026/tramtai-v0.50.1%2B19102026-linux-amd64.tar.gz"
      sha256 "449f6805362e52f634eb7c03f8bf114988d69cee6fb69244b96f01da0c71fc77"
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
