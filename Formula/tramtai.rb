# Generado por scripts/paquetes.ps1 al publicar v0.59.0+12112026; no lo edites a mano.
class Tramtai < Formula
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"
  version "0.59.0"
  license "PolyForm-Noncommercial-1.0.0"

  on_macos do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.59.0%2B12112026/tramtai-v0.59.0%2B12112026-macos-arm64.tar.gz"
      sha256 "1e674fc11c12bd3e5738b63c40f403dac9873625894cc03efec3d42a3d080827"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.59.0%2B12112026/tramtai-v0.59.0%2B12112026-macos-amd64.tar.gz"
      sha256 "2de9935aa072efddc72783457ce0743f15caf3b4afaa87b50e65107ef870d63e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.59.0%2B12112026/tramtai-v0.59.0%2B12112026-linux-arm64.tar.gz"
      sha256 "2334a29717010349aac839491c9665a43512b8fb95f6ec0fc2ca3112861ea81f"
    end
    on_intel do
      url "https://github.com/MscottB/Tramtai/releases/download/v0.59.0%2B12112026/tramtai-v0.59.0%2B12112026-linux-amd64.tar.gz"
      sha256 "43f93bff56cb1ce812c812b4f0954d382bde6c00b9ab0a29a4bb5d92ab3f16e4"
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
