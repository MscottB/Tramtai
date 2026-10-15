# Generado por scripts/paquetes.ps1 al publicar v0.29.0+15102026; no lo edites a mano.
cask "tramtai" do
  version "0.29.0"
  sha256 "b655be8484315a37b3a5ea0bef9578d3b8713f2d880a15f32f0e5839d684e9ff"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.29.0%2B15102026/Tramtai-v0.29.0%2B15102026-macos.dmg"
  name "Tramtai"
  desc "Tus agentes de IA (Claude Code, Codex, Gemini) como una API local compatible con OpenAI y Anthropic"
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
