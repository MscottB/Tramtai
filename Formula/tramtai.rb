# Generado por scripts/paquetes.ps1 al publicar v0.36.1+16102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.36.1"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.36.1%2B16102026/tramtai-v0.36.1%2B16102026-macos-arm64.tar.gz"
      sha256 "af3fc69e2e15503c05fab49df605e8f7bf9097415b22a3ccf5b0863f4cbe902b"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.36.1%2B16102026/tramtai-v0.36.1%2B16102026-macos-amd64.tar.gz"
      sha256 "00bd78aeb4d98db125339efefcd72032aed36ff70dd23918c56102c9a2507108"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.36.1%2B16102026/tramtai-v0.36.1%2B16102026-linux-arm64.tar.gz"
      sha256 "2984a4fc408786e6a2887b9fa77c1e04130facb713177f89397ee452852c0e9c"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.36.1%2B16102026/tramtai-v0.36.1%2B16102026-linux-amd64.tar.gz"
      sha256 "64d5424d05ded305ca3bc6d85502de79d7ffb724bb49bb48bc97454e453da744"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
