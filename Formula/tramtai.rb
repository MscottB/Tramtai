# Generado por scripts/paquetes.ps1 al publicar v0.33.2+15102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.33.2"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.2%2B15102026/tramtai-v0.33.2%2B15102026-macos-arm64.tar.gz"
      sha256 "cc0ca6caa5392a69ff1a6a45f0ac0146e315643f52f3d920dc60fc1d033428ad"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.2%2B15102026/tramtai-v0.33.2%2B15102026-macos-amd64.tar.gz"
      sha256 "38fb2eff1bb4e658736f69db5a2c6cb7986cf8b005ff3ca0672da8fd9d70207a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.2%2B15102026/tramtai-v0.33.2%2B15102026-linux-arm64.tar.gz"
      sha256 "daf0b9dd1177bfd3a982e583810fa76a832b6f3e51041872c9f55c910f0a418f"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.33.2%2B15102026/tramtai-v0.33.2%2B15102026-linux-amd64.tar.gz"
      sha256 "4854933ecdde28b475f36dc62189bd14ef62fdbbddd50188e19e91e3db927056"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
