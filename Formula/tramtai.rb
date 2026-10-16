# Generado por scripts/paquetes.ps1 al publicar v0.35.0+16102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.35.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.35.0%2B16102026/tramtai-v0.35.0%2B16102026-macos-arm64.tar.gz"
      sha256 "15fe20306577d798175db58468a0c63d345b684be54c1b7907029eeb4fdff02e"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.35.0%2B16102026/tramtai-v0.35.0%2B16102026-macos-amd64.tar.gz"
      sha256 "cb944f957cbf9ea79ccdcd5ca91033dbabe1c411546515d822cb3ceb32a0c401"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.35.0%2B16102026/tramtai-v0.35.0%2B16102026-linux-arm64.tar.gz"
      sha256 "d6831051589b13d80d14f33c6e3aae1966fb4cd0c0416722c7fdc087d976f167"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.35.0%2B16102026/tramtai-v0.35.0%2B16102026-linux-amd64.tar.gz"
      sha256 "493e338948ae2a2286d2a31dfe4c6d9bb7237fdc3db2286be634f6f3ff8c3bbb"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
