# Generado por scripts/paquetes.ps1 al publicar v0.51.0+20102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.51.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.51.0%2B20102026/tramtai-v0.51.0%2B20102026-macos-arm64.tar.gz"
      sha256 "2a9d41ff70639cec54953f7020e62b09cd6bfdfe4636b4f149facefc24f1fdc7"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.51.0%2B20102026/tramtai-v0.51.0%2B20102026-macos-amd64.tar.gz"
      sha256 "b2b987231368fd79d19d97077f86b21d812f1169de9389bde55594d24c1e4d93"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.51.0%2B20102026/tramtai-v0.51.0%2B20102026-linux-arm64.tar.gz"
      sha256 "1abefc815d7bdde0b2e46c5c6ee744f3407f11cabfcae629d9e3777f4f9cbb9e"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.51.0%2B20102026/tramtai-v0.51.0%2B20102026-linux-amd64.tar.gz"
      sha256 "a4c3450a6ee887696b1673e75b8423882d3dea7eb3bd9baf39939e1ee74c41ed"
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
