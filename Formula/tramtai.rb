# Generado por scripts/paquetes.ps1 al publicar v0.36.0+16102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.36.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.36.0%2B16102026/tramtai-v0.36.0%2B16102026-macos-arm64.tar.gz"
      sha256 "901cf137ed7f85dd9c570fe9efa3ed40cd2e45dc3cbd497822206f44c7e0f0dc"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.36.0%2B16102026/tramtai-v0.36.0%2B16102026-macos-amd64.tar.gz"
      sha256 "eca93a7a122fae1e6db6db91f097a2fbc7ca09fdac97f2a23892e8360b7da4a8"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.36.0%2B16102026/tramtai-v0.36.0%2B16102026-linux-arm64.tar.gz"
      sha256 "e2329e59ced0fec5f803445e814cada4431f773a1cde058612afc0cca0048f25"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.36.0%2B16102026/tramtai-v0.36.0%2B16102026-linux-amd64.tar.gz"
      sha256 "df79f942b94cd011db6e2baa9cb4d776da0fd5531a3c0d890e48551297fe7f14"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
