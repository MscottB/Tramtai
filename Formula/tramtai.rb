# Generado por scripts/paquetes.ps1 al publicar v0.56.0+26102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.56.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.56.0%2B26102026/tramtai-v0.56.0%2B26102026-macos-arm64.tar.gz"
      sha256 "da8b37e771937fc29c028f3bc1272a7000223bea68627f9909c3d4ba9ad0107b"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.56.0%2B26102026/tramtai-v0.56.0%2B26102026-macos-amd64.tar.gz"
      sha256 "40a5f6f46b2c16e3c56f002d13a2b2ecc5f058cf8dc435d1af46de1bd0cfba42"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.56.0%2B26102026/tramtai-v0.56.0%2B26102026-linux-arm64.tar.gz"
      sha256 "628adee09de89c2295e86107675e13abb67aac75e59533619e53a5353b9b919c"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.56.0%2B26102026/tramtai-v0.56.0%2B26102026-linux-amd64.tar.gz"
      sha256 "0848b65b493284cbc825f7f1c0c2fe3337af04643eeaadbaeb14f176bc63c0c6"
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
