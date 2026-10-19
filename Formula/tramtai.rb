# Generado por scripts/paquetes.ps1 al publicar v0.46.1+18102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.46.1"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.46.1%2B18102026/tramtai-v0.46.1%2B18102026-macos-arm64.tar.gz"
      sha256 "cb672d51cc71df40c2837e2fb8131093dc5182d1bc324bcef2fa04868c722ab0"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.46.1%2B18102026/tramtai-v0.46.1%2B18102026-macos-amd64.tar.gz"
      sha256 "00d9dfee01415f80b6868a6045038af6ed264d1b6032ed14eeb0f4ab13dc8027"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.46.1%2B18102026/tramtai-v0.46.1%2B18102026-linux-arm64.tar.gz"
      sha256 "6d61fc8ea151d9072b7fba2367113021a59e32683a689831dca6564ac2e69eb4"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.46.1%2B18102026/tramtai-v0.46.1%2B18102026-linux-amd64.tar.gz"
      sha256 "caa7ef8d026e7a6394d33080114facb19dc740f5ccb07411b55f708a0b2feea5"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
