# Generado por scripts/paquetes.ps1 al publicar v0.54.0+21102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.54.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.54.0%2B21102026/tramtai-v0.54.0%2B21102026-macos-arm64.tar.gz"
      sha256 "e92746b19d5b9b5f4b67632f60b40f09f9551d2b7fa6f341c4db29736e2f891a"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.54.0%2B21102026/tramtai-v0.54.0%2B21102026-macos-amd64.tar.gz"
      sha256 "8b28b62baf7d1e35d44f3d0ca4fe36c8ea373125e74c55d2502295e4fbdcfccc"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.54.0%2B21102026/tramtai-v0.54.0%2B21102026-linux-arm64.tar.gz"
      sha256 "53543cbae86ad104e32d60e02861f0100226b36a25310e9d7769e112d466b20a"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.54.0%2B21102026/tramtai-v0.54.0%2B21102026-linux-amd64.tar.gz"
      sha256 "de8d3b99bf361bbce2b2461f26d1d2d759d94a8c0a8503edf0a08ed54588a270"
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
