# Generado por scripts/paquetes.ps1 al publicar v0.30.0+15102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.30.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.30.0%2B15102026/tramtai-v0.30.0%2B15102026-macos-arm64.tar.gz"
      sha256 "eee0427bb2a256f84152b29f0c9c74a0af56e99737fda86865002bbe05cf26df"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.30.0%2B15102026/tramtai-v0.30.0%2B15102026-macos-amd64.tar.gz"
      sha256 "247eb27de2a86bbdab6494af25f22ff4c6c6839a9edd910907679ef8ee77947b"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.30.0%2B15102026/tramtai-v0.30.0%2B15102026-linux-arm64.tar.gz"
      sha256 "782650f32c37892234a468f89e9d8e0dbf0bb42f85c46899c4bd8bf6bced7fa7"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.30.0%2B15102026/tramtai-v0.30.0%2B15102026-linux-amd64.tar.gz"
      sha256 "2a6bcec5d7fe3e74aae3c1f5a2d3c32f5d1308f5a736f777916c88288d253af6"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
