# Generado por scripts/paquetes.ps1 al publicar v0.31.0+15102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.31.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.31.0%2B15102026/tramtai-v0.31.0%2B15102026-macos-arm64.tar.gz"
      sha256 "168c5e45b5487f7669ed0e64e5065a70065d62204d8b58ec78362c28440bf78b"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.31.0%2B15102026/tramtai-v0.31.0%2B15102026-macos-amd64.tar.gz"
      sha256 "58537d59e63742faee2d4fd90dac2db0af31fb149a49fef8b69009236ac7766a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.31.0%2B15102026/tramtai-v0.31.0%2B15102026-linux-arm64.tar.gz"
      sha256 "3f083fcc1f63e07c31c68135000b2df5369715da79199210fd422e692b15bc47"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.31.0%2B15102026/tramtai-v0.31.0%2B15102026-linux-amd64.tar.gz"
      sha256 "87957fb3c16ca5a37fe69219f8ad75488afec49941002adfd6f62bf39cb8a64f"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
