# Generado por scripts/paquetes.ps1 al publicar v0.58.0+31102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.58.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.58.0%2B31102026/tramtai-v0.58.0%2B31102026-macos-arm64.tar.gz"
      sha256 "52a4bce98781b94fde846d784a4a66bd2ae69b79a635e752c1e9b14db1cd691b"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.58.0%2B31102026/tramtai-v0.58.0%2B31102026-macos-amd64.tar.gz"
      sha256 "23801752586c68e28bbec2f17bd1fc6c8b935d0c21242e1fe817a71fba88839f"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.58.0%2B31102026/tramtai-v0.58.0%2B31102026-linux-arm64.tar.gz"
      sha256 "d1838b26e484cd2c404f6f1f1d19a17129e23efa09ed7679aad4954fcb6e1bac"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.58.0%2B31102026/tramtai-v0.58.0%2B31102026-linux-amd64.tar.gz"
      sha256 "db424f5c1bf2086a91fc968f8e97b454c5276d3e577017f60a7f9216fc7184fe"
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
