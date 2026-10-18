# Generado por scripts/paquetes.ps1 al publicar v0.42.0+17102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.42.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.42.0%2B17102026/tramtai-v0.42.0%2B17102026-macos-arm64.tar.gz"
      sha256 "62791545fbf48cc896c926fe3319a7ee292b60a86bde9778e2a962753cf7e982"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.42.0%2B17102026/tramtai-v0.42.0%2B17102026-macos-amd64.tar.gz"
      sha256 "c41d9914536e56037610e91902459f3d6e6635c662ff5b20e5dd3c13861b695b"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.42.0%2B17102026/tramtai-v0.42.0%2B17102026-linux-arm64.tar.gz"
      sha256 "83ffe96b66a367ef12c5fc7740cd2497bd9c27676b633e5f6880fdf1fe5d973c"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.42.0%2B17102026/tramtai-v0.42.0%2B17102026-linux-amd64.tar.gz"
      sha256 "3e5b5df40ec3280b7a69bf9238292be7bbf611dba7265b0fd60f2e37402a7a63"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
