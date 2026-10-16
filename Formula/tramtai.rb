# Generado por scripts/paquetes.ps1 al publicar v0.34.0+16102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.34.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.34.0%2B16102026/tramtai-v0.34.0%2B16102026-macos-arm64.tar.gz"
      sha256 "29ab25cfbc9a0a9c979fedb7f3c9dcecabe076a6a1d4483311b7afeb689fe0d1"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.34.0%2B16102026/tramtai-v0.34.0%2B16102026-macos-amd64.tar.gz"
      sha256 "74809911c661015450816d34eca4708686d9e65744a1d05d923440f078e10289"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.34.0%2B16102026/tramtai-v0.34.0%2B16102026-linux-arm64.tar.gz"
      sha256 "b36fa73596a3f64a142571ef44463a0f289eb2ad2c3e3fd0febfd5cdc2771a71"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.34.0%2B16102026/tramtai-v0.34.0%2B16102026-linux-amd64.tar.gz"
      sha256 "b1f015eccbd12f8d22beb0eaf26bec9dc1ca5873f6992549ce413d04f18229b3"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
