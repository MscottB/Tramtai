# Generado por scripts/paquetes.ps1 al publicar v0.32.0+15102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.32.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.32.0%2B15102026/tramtai-v0.32.0%2B15102026-macos-arm64.tar.gz"
      sha256 "7aa47ac218279c6c4954d8753241e5688efc61ade8530af2687a06c26fb69af0"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.32.0%2B15102026/tramtai-v0.32.0%2B15102026-macos-amd64.tar.gz"
      sha256 "baf9178eeef6cb478690cc71176ed282f836b7c01825b1080537660b61bd45b8"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.32.0%2B15102026/tramtai-v0.32.0%2B15102026-linux-arm64.tar.gz"
      sha256 "33b1c7f0e2a56d0b6ede24f16a7dc39f63f141dd5079495413555d2bd84ad62f"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.32.0%2B15102026/tramtai-v0.32.0%2B15102026-linux-amd64.tar.gz"
      sha256 "a075d2970e856729abfeca6b7123a38b6a24ff81bcae6b6408587bed1112eff2"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
