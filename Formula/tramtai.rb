# Generado por scripts/paquetes.ps1 al publicar v0.35.1+16102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.35.1"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.35.1%2B16102026/tramtai-v0.35.1%2B16102026-macos-arm64.tar.gz"
      sha256 "598a619a5bcd71b25967df77725bb1a0208544bbc5658a4c19ab7338df5b1e51"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.35.1%2B16102026/tramtai-v0.35.1%2B16102026-macos-amd64.tar.gz"
      sha256 "6062d7237c7b9dcf521b436d016f271d4082437d104f0479a66008dcfa5df26b"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.35.1%2B16102026/tramtai-v0.35.1%2B16102026-linux-arm64.tar.gz"
      sha256 "1bbc557c4ce5f0bfdfde2a662d4589698e9387e368522bb7ade46971c33466a9"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.35.1%2B16102026/tramtai-v0.35.1%2B16102026-linux-amd64.tar.gz"
      sha256 "a99d9123d85b27515f4280c3419e2349b7fb6d09afa8e516276e1001f3ae2b56"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
