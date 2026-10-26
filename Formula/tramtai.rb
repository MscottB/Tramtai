# Generado por scripts/paquetes.ps1 al publicar v0.57.0+26102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.57.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.57.0%2B26102026/tramtai-v0.57.0%2B26102026-macos-arm64.tar.gz"
      sha256 "eb2047cc9351ddaf523375fa3cc54648c22e8c9720c37474f2f33f3ffd5b3349"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.57.0%2B26102026/tramtai-v0.57.0%2B26102026-macos-amd64.tar.gz"
      sha256 "378a3e11d77ae48ad6e53598bd2aca17c986e489c5667d6034bbb60fbfbf8f06"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.57.0%2B26102026/tramtai-v0.57.0%2B26102026-linux-arm64.tar.gz"
      sha256 "068f17e052c7178f4f3745cd98db63c83e575e3597ed2824ef88f2c6913a9fd3"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.57.0%2B26102026/tramtai-v0.57.0%2B26102026-linux-amd64.tar.gz"
      sha256 "7347113ecedd617baa712e40e345d17a8001eb5c8f52fb8daad98afca7a78d94"
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
