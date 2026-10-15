# Generado por scripts/paquetes.ps1 al publicar v0.33.1+15102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.33.1"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.1%2B15102026/tramtai-v0.33.1%2B15102026-macos-arm64.tar.gz"
      sha256 "264c6599590eaf2851bfdaa492e25dd918197d29e5633ad68883d1ca6e2362d3"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.1%2B15102026/tramtai-v0.33.1%2B15102026-macos-amd64.tar.gz"
      sha256 "2e736265509461847fd2d8462c28dd66d950ec0d01bf708f0082bf4dc69b9054"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.1%2B15102026/tramtai-v0.33.1%2B15102026-linux-arm64.tar.gz"
      sha256 "040405681e802435a89fb5f7d5c17033cab67110ada09dfdc575a191b1922eca"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.1%2B15102026/tramtai-v0.33.1%2B15102026-linux-amd64.tar.gz"
      sha256 "08a6c0aa8397ed4d0cfb082ae9ce12673c24eb3f238f7ae6d7e46eee3576491a"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
