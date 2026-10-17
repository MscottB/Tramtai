# Generado por scripts/paquetes.ps1 al publicar v0.39.0+17102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.39.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.39.0%2B17102026/tramtai-v0.39.0%2B17102026-macos-arm64.tar.gz"
      sha256 "bbf40b6b511de7f460fad69b9d3296e352b8254989da08530d86738cdcfd37ef"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.39.0%2B17102026/tramtai-v0.39.0%2B17102026-macos-amd64.tar.gz"
      sha256 "da5d18dd1f0b3c528d37627aef50f4e2300ad458baf71d8aaebcb04a4a2d25bc"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.39.0%2B17102026/tramtai-v0.39.0%2B17102026-linux-arm64.tar.gz"
      sha256 "0e21eb78549aa5bf4668643a81fd4ad90c86926ec4a2debde117029a330b8b35"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.39.0%2B17102026/tramtai-v0.39.0%2B17102026-linux-amd64.tar.gz"
      sha256 "2ed6c4e98d5c5d853bf86d917b46c15469558f267009e6ad04befaf2f6e13074"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
