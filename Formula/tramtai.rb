# Generado por scripts/paquetes.ps1 al publicar v0.49.1+19102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.49.1"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.49.1%2B19102026/tramtai-v0.49.1%2B19102026-macos-arm64.tar.gz"
      sha256 "209ad0144d1729d6410c4e0b14c0c5dc72e0bf37204a53e3a8caccd1998872ae"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.49.1%2B19102026/tramtai-v0.49.1%2B19102026-macos-amd64.tar.gz"
      sha256 "3d44571ef858c7fd291eba9357f52a33fa0979c2e876aeef3d3e79d16ab2e58e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.49.1%2B19102026/tramtai-v0.49.1%2B19102026-linux-arm64.tar.gz"
      sha256 "ecdca419d3154d84fa0a4fcaa6632bb9bb72cd3a90f6eaa3b7d5cb476f64b154"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.49.1%2B19102026/tramtai-v0.49.1%2B19102026-linux-amd64.tar.gz"
      sha256 "b9d7e12fe1c02df8f22ce02aec7efc948d7f04a059cb8e3b2bc37710d2b03ab1"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
