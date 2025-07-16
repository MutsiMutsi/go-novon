<script>
  import logo from "./assets/images/logo.svg";
  import obsConfig from "./assets/images/obs_config.png";

  import { StartStream, StopStream } from "../wailsjs/go/main/App.js";
  import { EventsOn } from "../wailsjs/runtime";

  EventsOn("UpdateEvent", (data) => {
    switch (data.Type) {
      case "NKN_UPDATE":
        nknStatus.clients = data.NumClients;
        nknStatus.status = data.Status;
        break;
      case "RTMP_PORT":
        rtmpState = "Waiting";
        rtmpPort = data.value;
        break;
      case "RTMP_PUBLISH":
        rtmpState = "Streaming";
        break;
      case "PUBLISH":
        rtmpState = "Streaming";
        publishStatus = data;
        break;
      case "RTMP_TERMINATED":
        rtmpState = "Offline";
        break;
      case "FFMPEG_UPDATE":
        console.log(data);
        if (data.IsInstalled === "FALSE") {
          showDialog = true;

          switch (data.OS) {
            case "windows":
              installInstructions = `
              1. Download FFmpeg from https://www.gyan.dev/ffmpeg/builds/
              2. Extract it and place it in a permanent location (e.g., C:\\ffmpeg)
              3. Add the 'bin' folder to your system PATH.
            `;
              installUrl = "https://www.gyan.dev/ffmpeg/builds/";
              break;
            case "darwin":
              installInstructions = `
              1. Install Homebrew from https://brew.sh (if not installed)
              2. Run: brew install ffmpeg
            `;
              installUrl = "https://brew.sh/";
              break;
            case "linux":
              installInstructions = `
              Install FFmpeg using your package manager:
              - Debian/Ubuntu: sudo apt install ffmpeg
              - Fedora: sudo dnf install ffmpeg
              - Arch: sudo pacman -S ffmpeg
            `;
              installUrl = "https://ffmpeg.org/download.html";
              break;
            default:
              installInstructions =
                "Please install FFmpeg from https://ffmpeg.org/download.html";
              installUrl = "https://ffmpeg.org/download.html";
          }
        }
        break;
      default:
        console.log(data);
        break;
    }
  });

  let showDialog = false;
  let installUrl = "";
  let installInstructions = "";

  let streamActive = false;
  let rtmpState = "Offline";
  let rtmpPort = "";

  let nknStatus = {
    clients: 0,
    status: "Offline",
    pubkey: "not connected",
    wallet: "not connected",
  };

  let publishStatus = {
    numViewers: 0,
    segmentSize: 0,
    numChunks: 0,
  };

  async function startBackend() {
    const result = await StartStream();
    if (result.Error) {
      alert(result.Error);
      return;
    }

    nknStatus.wallet = result.Wallet;
    nknStatus.pubkey = result.Address;
    streamActive = true;
  }

  async function stopBackend() {
    await StopStream();
    streamActive = false;
    nknStatus.clients = 0;
    nknStatus.status = "Stopped";
  }

  function openInstallPage() {
    window.open(installUrl, "_blank");
  }
</script>

<main>
  <div class="container" style="padding: 2rem 1rem;">
    <div class="row">
      <div class="twelve columns">
        <img alt="Wails logo" id="logo" src={logo} style="width: 64px;" />
        <h3>GoNovon Dashboard</h3>
        <hr />
      </div>
    </div>

    {#if showDialog}
      <div class="row">
        <div class="ffmpeg-dialog">
          <h2 class="text-xl">⚠️ FFmpeg is required</h2>
          <p class="text-sm">
            FFmpeg is not currently installed on your system. Please follow the
            instructions below to install it:
          </p>
          <pre style="color: #868686;">
{installInstructions}
    </pre>
          <button
            on:click={openInstallPage}
            class="text-white px-4 py-2 rounded hover:bg-blue-700"
          >
            Open FFmpeg Install Page
          </button>
        </div>
      </div>
    {:else}
      <div class="row">
        <!-- Stream Server Status -->
        <div class="six columns">
          <h5>📡 Streaming Server</h5>
          <p>
            <strong>Status:</strong>
            {rtmpState}
          </p>
          {#if rtmpState != "Streaming"}
            <p><strong>Example OBS Config:</strong></p>
            <img
              alt="Wails logo"
              id="logo"
              src={obsConfig}
              style="width: 100%;"
            />
          {/if}
          {#if rtmpState == "Streaming"}
            <p>
              <strong>Viewers:</strong>
              <code>{publishStatus.numViewers}</code>
            </p>
            <p>
              <strong>Segment Size:</strong>
              <code>{publishStatus.segmentSize}</code>
            </p>
            <p>
              <strong>Segment Chunks:</strong>
              <code>{publishStatus.numChunks}</code>
            </p>
          {/if}
        </div>

        <!-- NKN Node Status -->
        <div class="six columns">
          <h5>🖥️ NKN Node</h5>
          <p>
            <strong>Clients Connected:</strong>
            {nknStatus.clients}/96
          </p>
          <p>
            <strong>Public Key:</strong><br /> <code>{nknStatus.pubkey}</code>
          </p>
          <p><strong>Wallet:</strong><br /> <code>{nknStatus.wallet}</code></p>
        </div>
      </div>

      <hr />

      <div class="row">
        <div class="twelve columns">
          {#if !streamActive && nknStatus.status != "Starting"}
            <button class="button u-full-width" on:click={startBackend}>
              ▶️ Start GoNovon
            </button>
          {/if}
          {#if streamActive}
            <button
              class="button u-full-width"
              on:click={stopBackend}
              style="margin-top: 1rem;"
            >
              ⏹ Stop GoNovon
            </button>
          {/if}
        </div>
      </div>
    {/if}
  </div>
</main>

<style>
  main {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
      "Helvetica Neue", sans-serif;
    color: #333;
  }

  #logo {
    display: block;
    margin: auto;
  }

  code {
    background: #141414;
    padding: 0.2rem 0.4rem;
    border-radius: 3px;
    font-family: monospace;
  }
</style>
