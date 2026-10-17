# Generado por scripts/paquetes.ps1 al publicar v0.38.0+17102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.38.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.38.0%2B17102026/tramtai-v0.38.0%2B17102026-macos-arm64.tar.gz"
      sha256 "dfd094735d6619e9ffc75f3947d96bf7886cba834f451eb7cd9c263f86f329a0"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.38.0%2B17102026/tramtai-v0.38.0%2B17102026-macos-amd64.tar.gz"
      sha256 "53a879cc685d2a58bc262faf88da8d1ec7d15bb38f4c48ea9076d303652e0e82"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.38.0%2B17102026/tramtai-v0.38.0%2B17102026-linux-arm64.tar.gz"
      sha256 "5f92fc6f665e8f3ab3c9c0b0452fe60e82d98478f86c4e9480c6532a99c5df65"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.38.0%2B17102026/tramtai-v0.38.0%2B17102026-linux-amd64.tar.gz"
      sha256 "adbad7275146e54e2f499c3ad6b5cc03da2a91796ce8e306a3cb2db060ce8ce3"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
