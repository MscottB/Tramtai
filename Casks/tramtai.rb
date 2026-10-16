# Generado por scripts/paquetes.ps1 al publicar v0.36.1+16102026; no lo edites a mano.
cask "tramtai" do
  version "0.36.1"
  sha256 "3b0bc9b8e7360ccd801039159c0fd4b3767f356c93eadb0b75d5502c337aae1c"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.36.1%2B16102026/Tramtai-v0.36.1%2B16102026-macos.dmg"
  name "Tramtai"
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"

  depends_on macos: ">= :big_sur"

  app "Tramtai.app"
  binary "#{appdir}/Tramtai.app/Contents/MacOS/tramtai"

  # La app no está firmada por un desarrollador de Apple: sin la marca de cuarentena macOS la abre sin bloquearla.
  postflight do
    system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{appdir}/Tramtai.app"]
  end

  uninstall launchctl: "com.mscottb.tramtai",
            quit:      "com.mscottb.tramtai"

  zap trash: [
    "~/Library/Application Support/tramtai",
    "~/Library/LaunchAgents/com.mscottb.tramtai.plist",
  ]
end
