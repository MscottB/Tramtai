# Generado por scripts/paquetes.ps1 al publicar v0.45.0+18102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.45.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.45.0%2B18102026/tramtai-v0.45.0%2B18102026-macos-arm64.tar.gz"
      sha256 "1cec458c2ca3333613580124b1c4a94a10c2b32437c3d452a3b29674dfb7d476"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.45.0%2B18102026/tramtai-v0.45.0%2B18102026-macos-amd64.tar.gz"
      sha256 "0cf0d1dc9f038fe39e773e8fbedc7f7d165e07f95aaacfef2371766d3cf5c791"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.45.0%2B18102026/tramtai-v0.45.0%2B18102026-linux-arm64.tar.gz"
      sha256 "fe1a768853dce418197dac6c6b8f24d3344d8f97b3f35f77390919c0e03e4dd2"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.45.0%2B18102026/tramtai-v0.45.0%2B18102026-linux-amd64.tar.gz"
      sha256 "9d7e26fed0b87a862d04a923ce0fe01dc1838204ec4da5ea27f39646786d212e"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
