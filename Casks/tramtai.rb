# Generado por scripts/paquetes.ps1 al publicar v0.30.0+15102026; no lo edites a mano.
cask "tramtai" do
  version "0.30.0"
  sha256 "f8b8f8d20c0a55e7ce2e62e678e70123ab5cdf71d4bd8cc42da06a9bb69fc4e3"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.30.0%2B15102026/Tramtai-v0.30.0%2B15102026-macos.dmg"
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
