# Generado por scripts/paquetes.ps1 al publicar v0.29.0+15102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.29.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.29.0%2B15102026/tramtai-v0.29.0%2B15102026-macos-arm64.tar.gz"
      sha256 "45208ab0f75e382893ae0db403c2d1a1b4b7f3204809c297e3b428ff15142742"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.29.0%2B15102026/tramtai-v0.29.0%2B15102026-macos-amd64.tar.gz"
      sha256 "515b4579869d1dc61d63e79795c0c3cd0b9d445d5d6c61360690fe45ade795a8"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.29.0%2B15102026/tramtai-v0.29.0%2B15102026-linux-arm64.tar.gz"
      sha256 "320ee695024693c8f6548cea3480f7a285ecc29bb7fa5ed30f2ce1d1a0ebd6ad"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.29.0%2B15102026/tramtai-v0.29.0%2B15102026-linux-amd64.tar.gz"
      sha256 "d2599242a4594ca8e3f9025f3343e67f26cedb0fae66a946e3b9370d50a5fda6"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
