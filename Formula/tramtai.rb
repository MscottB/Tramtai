# Generado por scripts/paquetes.ps1 al publicar v0.53.0+20102026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.53.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.53.0%2B20102026/tramtai-v0.53.0%2B20102026-macos-arm64.tar.gz"
      sha256 "307c8470fcfe964b0e8740746e01103fc744c8139d69d630f88d064e3a766dd2"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.53.0%2B20102026/tramtai-v0.53.0%2B20102026-macos-amd64.tar.gz"
      sha256 "135219bcd14686b856c60f4c7808b25cf6fca085cf09e6696c58e349845f72cd"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.53.0%2B20102026/tramtai-v0.53.0%2B20102026-linux-arm64.tar.gz"
      sha256 "85c4692bf7d2ff22f17e2390ad6a7f1ea0310d146c4c8206f2f4040a67be7849"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.53.0%2B20102026/tramtai-v0.53.0%2B20102026-linux-amd64.tar.gz"
      sha256 "73d27fb3f4e73aa7698ec6dace1f38e37576a06867e1f5e7750655cd57a204d7"
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
