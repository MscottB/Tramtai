# Generado por scripts/paquetes.ps1 al publicar v0.44.0+18102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.44.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.44.0%2B18102026/tramtai-v0.44.0%2B18102026-macos-arm64.tar.gz"
      sha256 "b0cb91352118da8fda6c2003ab2d91e260ec0460940165df1e26bc51a0b88d69"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.44.0%2B18102026/tramtai-v0.44.0%2B18102026-macos-amd64.tar.gz"
      sha256 "fd334145ad6ffea738dd3b2b069d0eb4084c7da441cd702a3f6374932ef9d3c2"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.44.0%2B18102026/tramtai-v0.44.0%2B18102026-linux-arm64.tar.gz"
      sha256 "6bdd490c931c8e7b47663a6209263e4ef2c6d5fc3d9e98b9bd6059e18d172b90"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.44.0%2B18102026/tramtai-v0.44.0%2B18102026-linux-amd64.tar.gz"
      sha256 "441cbd55ba6235169bea1a57d19a2cd7f8c1d4162d630d560b672125ed3d4373"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
