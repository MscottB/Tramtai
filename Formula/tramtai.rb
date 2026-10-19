# Generado por scripts/paquetes.ps1 al publicar v0.47.0+19102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.47.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.47.0%2B19102026/tramtai-v0.47.0%2B19102026-macos-arm64.tar.gz"
      sha256 "9da7e0e865ecf9c180eb9b89dad362c370c7661816fb070e34bf045c943c44a2"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.47.0%2B19102026/tramtai-v0.47.0%2B19102026-macos-amd64.tar.gz"
      sha256 "e6956e794c74ae1069b89d814e96d6bb2b83f2aabc6a6334e5dfc00238bc147b"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.47.0%2B19102026/tramtai-v0.47.0%2B19102026-linux-arm64.tar.gz"
      sha256 "8c394b6d714cdfbdfacb3136e45f4e570f40c4b07df33ea85af7c35cd049b114"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.47.0%2B19102026/tramtai-v0.47.0%2B19102026-linux-amd64.tar.gz"
      sha256 "35e397a7d6f91ba0e271a84a6fd2426b3a163a6822e2862e86b11bb674db540e"
    end
  end

  def install
    bin.install "tramtai"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
