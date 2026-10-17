# Generado por scripts/paquetes.ps1 al publicar v0.39.0+17102026; no lo edites a mano.
cask "tramtai" do
  version "0.39.0"
  sha256 "20b4ff2bad5bb4de9a1f9d0ca8f77fdf6ff7d789b1cadf3cc414fba79d847d07"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.39.0%2B17102026/Tramtai-v0.39.0%2B17102026-macos.dmg"
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
