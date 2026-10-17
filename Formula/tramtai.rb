# Generado por scripts/paquetes.ps1 al publicar v0.37.0+16102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.37.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.37.0%2B16102026/tramtai-v0.37.0%2B16102026-macos-arm64.tar.gz"
      sha256 "610b18b602e2c0b4f81aee8d25b357b0c458ff04124dce8323e1816ad6fc2c39"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.37.0%2B16102026/tramtai-v0.37.0%2B16102026-macos-amd64.tar.gz"
      sha256 "9721e0e365d4535c15a97c8a68cbcb4a67ef123b4f86ed1a232c592cdebfe0cd"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.37.0%2B16102026/tramtai-v0.37.0%2B16102026-linux-arm64.tar.gz"
      sha256 "8497501e83655c3f99cc5dd0cabece7a0c466ebcd9f4b280c954f6987a26c4f7"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.37.0%2B16102026/tramtai-v0.37.0%2B16102026-linux-amd64.tar.gz"
      sha256 "d817957d18167dccd82e8ac61e6e932ccb3bd7ddf619ce0ef3a818cf4ff35b7d"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
