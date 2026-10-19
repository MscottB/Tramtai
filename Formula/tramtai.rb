# Generado por scripts/paquetes.ps1 al publicar v0.50.0+19102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.50.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.0%2B19102026/tramtai-v0.50.0%2B19102026-macos-arm64.tar.gz"
      sha256 "089523c5e5c1c3416015c431bfa45abf1f0259f43a988b7f5122ef9ba1a2f17c"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.0%2B19102026/tramtai-v0.50.0%2B19102026-macos-amd64.tar.gz"
      sha256 "d199e46c05fb3bd5897512426a7db4e5dee5bac151da9e535c8b98315c2b3424"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.0%2B19102026/tramtai-v0.50.0%2B19102026-linux-arm64.tar.gz"
      sha256 "83d833f9ba63cb9eb29a8921c5851fb6131c3926ee7a2c11141d131bccf7191b"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.50.0%2B19102026/tramtai-v0.50.0%2B19102026-linux-amd64.tar.gz"
      sha256 "9b98aaa2237e81689e6000d94bd7a0c85e62b85c9d13d80271488793e50d769a"
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
