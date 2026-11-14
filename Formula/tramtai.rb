# Generado por scripts/paquetes.ps1 al publicar v0.60.0+13112026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.60.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.60.0%2B13112026/tramtai-v0.60.0%2B13112026-macos-arm64.tar.gz"
      sha256 "233b49b175a6fa69383454147ceb35cd9407b5681ead6869db0393574bc5712a"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.60.0%2B13112026/tramtai-v0.60.0%2B13112026-macos-amd64.tar.gz"
      sha256 "72744303e8a052fcc4c3d4abaf4ce279ded9882dbc02500982c398ac31c76cf6"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.60.0%2B13112026/tramtai-v0.60.0%2B13112026-linux-arm64.tar.gz"
      sha256 "e0dd00e87602fc0011a6a4cef6e9e464ae3344100a3245a51aa4d8736f0c833c"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.60.0%2B13112026/tramtai-v0.60.0%2B13112026-linux-amd64.tar.gz"
      sha256 "ad39e53f81f6418027d26bbc434c17c6bc2c83e09c3b79edb0b91a3d9b45f4d7"
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
