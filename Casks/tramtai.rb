# Generado por scripts/paquetes.ps1 al publicar v0.52.0+20102026; no lo edites a mano.
cask "tramtai" do
  version "0.52.0"
  sha256 "68b1b53dc821f8ed94dda1d7680261c86e87b968372f6d9a88fb7cc9b31f7a21"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.52.0%2B20102026/Tramtai-v0.52.0%2B20102026-macos.dmg"
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
