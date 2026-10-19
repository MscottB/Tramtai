# Generado por scripts/paquetes.ps1 al publicar v0.48.0+19102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.48.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.48.0%2B19102026/tramtai-v0.48.0%2B19102026-macos-arm64.tar.gz"
      sha256 "745870906070b7888a24fb9a690552dbab72a584eeaa60dc58e87c38f5f64b22"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.48.0%2B19102026/tramtai-v0.48.0%2B19102026-macos-amd64.tar.gz"
      sha256 "56249686c1ab73a83e9b2506b81cde257c375121a85d779ad6c0936577432dd3"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.48.0%2B19102026/tramtai-v0.48.0%2B19102026-linux-arm64.tar.gz"
      sha256 "88e5d5202848f24bc15ff1c53f15529f04e7456736c31a8d26b1985253b8401a"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.48.0%2B19102026/tramtai-v0.48.0%2B19102026-linux-amd64.tar.gz"
      sha256 "e7c3494648967e0aa373db5bb361d58529f6f3e72db5676f475eb6963e970f8b"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
