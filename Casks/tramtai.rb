# Generado por scripts/paquetes.ps1 al publicar v0.50.1+19102026; no lo edites a mano.
cask "tramtai" do
  version "0.50.1"
  sha256 "295cece215c5e75bb51343ad64591490f2e647f0f48c8ecbb3bbdc2c4cf5ead2"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.50.1%2B19102026/Tramtai-v0.50.1%2B19102026-macos.dmg"
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
