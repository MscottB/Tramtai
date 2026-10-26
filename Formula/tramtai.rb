# Generado por scripts/paquetes.ps1 al publicar v0.56.1+26102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.56.1"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.56.1%2B26102026/tramtai-v0.56.1%2B26102026-macos-arm64.tar.gz"
      sha256 "e2bc45dc8310d7bc9e3ed396508f33ddeadf76f99f2fe1b2f3ae4c15811fe2a7"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.56.1%2B26102026/tramtai-v0.56.1%2B26102026-macos-amd64.tar.gz"
      sha256 "53994d1a25aea457c474f3cd4c24b95c36158c77f400a5240211dbb660173cc1"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.56.1%2B26102026/tramtai-v0.56.1%2B26102026-linux-arm64.tar.gz"
      sha256 "7d25995de61b41ed57be9c1af3d1cb5b51b0c5ee83a6117f1945e0e1cd1dc62e"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.56.1%2B26102026/tramtai-v0.56.1%2B26102026-linux-amd64.tar.gz"
      sha256 "086d7859a2ab3e22b5345a1c2e824f23699cca2101de63bd513ba15f1a1da953"
    end
  end

  def install
    bin.install "tramtai"
    (lib/"tramtai").install "uv" # prepara el conversor de archivos; Tramtai lo busca en ../lib/tramtai
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tramtai version")
  end
end
