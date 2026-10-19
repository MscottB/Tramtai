# Generado por scripts/paquetes.ps1 al publicar v0.46.0+18102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.46.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.46.0%2B18102026/tramtai-v0.46.0%2B18102026-macos-arm64.tar.gz"
      sha256 "b072d4d74cc87fbbf84d5b320065634bf6b081edc3290e5ba5a4650f55232503"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.46.0%2B18102026/tramtai-v0.46.0%2B18102026-macos-amd64.tar.gz"
      sha256 "39abfd232b07c70415a06e6dad84da2927caf5eec7f79034735e2d4694fbcd28"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.46.0%2B18102026/tramtai-v0.46.0%2B18102026-linux-arm64.tar.gz"
      sha256 "67c9fb5f7fe8586764f0c17aa44de8195980d77e3734b028732cb0fa9d6a94d5"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.46.0%2B18102026/tramtai-v0.46.0%2B18102026-linux-amd64.tar.gz"
      sha256 "b47b07a31dc563cecc0c127957ac552d052bb02747cef3d058e07d517b8763c6"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
