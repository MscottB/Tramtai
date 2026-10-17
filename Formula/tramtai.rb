# Generado por scripts/paquetes.ps1 al publicar v0.40.0+17102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.40.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.40.0%2B17102026/tramtai-v0.40.0%2B17102026-macos-arm64.tar.gz"
      sha256 "1c397b15773c6eefb43587de2e7103ccd2e1d47c87ba5764852f1c9d202e37f3"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.40.0%2B17102026/tramtai-v0.40.0%2B17102026-macos-amd64.tar.gz"
      sha256 "4ea9b94b758728e94d56d3a38da6f41de8bff7f9a72899514b0a2d7e0a9fc537"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.40.0%2B17102026/tramtai-v0.40.0%2B17102026-linux-arm64.tar.gz"
      sha256 "658875c2b23bd02d834f9d3b161993e15e5321918535264a2f8df7567a5ba57f"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.40.0%2B17102026/tramtai-v0.40.0%2B17102026-linux-amd64.tar.gz"
      sha256 "d223bbf6ad6c7f45a2de8300f631cdf20ebbac0b394b202020db261658c6f05c"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
