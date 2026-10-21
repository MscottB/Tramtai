# Generado por scripts/paquetes.ps1 al publicar v0.55.0+21102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.55.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.55.0%2B21102026/tramtai-v0.55.0%2B21102026-macos-arm64.tar.gz"
      sha256 "a2d6dcd83ca2fbd8ce6617aa8211706f79d554921fdf06a828a7f72961f4d677"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.55.0%2B21102026/tramtai-v0.55.0%2B21102026-macos-amd64.tar.gz"
      sha256 "a1bfc3ccf5b264ff0697f3a4328ffd66beb0bc388c4398c50a05f61abc34cb82"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.55.0%2B21102026/tramtai-v0.55.0%2B21102026-linux-arm64.tar.gz"
      sha256 "b94ea12a69749f89bd6ce864209662801552e27bd35aa1355f543531100d0396"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.55.0%2B21102026/tramtai-v0.55.0%2B21102026-linux-amd64.tar.gz"
      sha256 "1a98bc040edd141a94cceb111da6ca19e119fc36c05c6d2565ffd06a55728690"
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
