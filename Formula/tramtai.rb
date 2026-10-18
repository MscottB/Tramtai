# Generado por scripts/paquetes.ps1 al publicar v0.43.0+18102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.43.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.43.0%2B18102026/tramtai-v0.43.0%2B18102026-macos-arm64.tar.gz"
      sha256 "5ee86d351294b0db55a6455c2334e29fceed857391d6593a326a99be78f0f458"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.43.0%2B18102026/tramtai-v0.43.0%2B18102026-macos-amd64.tar.gz"
      sha256 "95159076756055705fca7abcbb95193e785885ba56b2df01cb855ea410fb8920"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.43.0%2B18102026/tramtai-v0.43.0%2B18102026-linux-arm64.tar.gz"
      sha256 "bb64345c98b912c5e955505052f8fcf69dfad36baeb17eee85f008a54e089e35"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.43.0%2B18102026/tramtai-v0.43.0%2B18102026-linux-amd64.tar.gz"
      sha256 "bbb36ece47397a3fb7f438154266bfbf45ba3b858dc38d17546fbc35def92a5c"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
