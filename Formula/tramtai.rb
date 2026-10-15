# Generado por scripts/paquetes.ps1 al publicar v0.33.0+15102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.33.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.0%2B15102026/tramtai-v0.33.0%2B15102026-macos-arm64.tar.gz"
      sha256 "2f5adee722564c45c771335ff4b50d86c89470e07527aabc5b40d4bc7d39a76b"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.0%2B15102026/tramtai-v0.33.0%2B15102026-macos-amd64.tar.gz"
      sha256 "656e9df946f32120e72652c4ad5f5b807a8dc683beb6bea26b450ff677403897"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.0%2B15102026/tramtai-v0.33.0%2B15102026-linux-arm64.tar.gz"
      sha256 "9f810d0d1bc673df45454b7c21097fc8e01893121c33b4deb63ea9784546a266"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.0%2B15102026/tramtai-v0.33.0%2B15102026-linux-amd64.tar.gz"
      sha256 "09fa54bbc4fa87488d33ed5aa175c680ad216016522f90716909d2bef91973e1"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
