# Mic-State Connector

    Trigger Loom jobs on meeting start and end using system microphone state as the signal — without requiring OpenWhispr or any dedicated meeting app.

    ## Background

    When Zoom, Teams, Google Meet, or any conferencing app grabs the microphone, the OS audio session state changes. When the call ends and the mic is released, the state reverts. This is more reliable than process detection (false positives from background apps) and more general than calendar-based detection (works without OAuth, fires on actual usage not scheduled time).

    Loom's connector pattern — poll an external source, diff against a mirror, fire jobs on change — maps naturally to this. The tradeoff is polling latency (3–5s) versus real-time event-driven detection, which is acceptable for meeting lifecycle automation.

    ## Connector Setup

    **Entity:** `system.mic/default`

    **State shape:**
    ```json
    {
      "state": "active | inactive",
      "since": "ISO8601"
    }
    ```

    ```bash
    loom connector add \
      --name "mic-state" \
      --method command \
      --command "<platform-command>" \
      --entity "system.mic/default" \
      --prompt "Track mic state: { state: 'active' | 'inactive', since: ISO8601 timestamp of last transition }. Active means another application is currently using the microphone." \
      --interval 4s
    ```

    ## Platform Commands

    ### macOS

    ```bash
    ioreg -l -w 0 -c IOAudioEngine 2>/dev/null \
      | grep -A5 'IOAudioEngineDirection.*Input' \
      | grep -c '"IOAudioEngineState" = 1' \
      | awk '{print ($1>0 ? "active" : "inactive")}'
    ```

    ### Windows

    ```powershell
    powershell -NoProfile -Command "
      if (Get-Process | Where-Object { $_.ProcessName -match 'zoom|teams|webex|meet' }) {
        'active'
      } else {
        'inactive'
      }
    "
    ```

    Note: degrades to process detection on Windows. A dedicated WASAPI query binary (analogous to OpenWhispr's `windows-mic-listener.exe`) would be more reliable.

    ### Linux

    ```bash
    pactl list source-outputs short 2>/dev/null \
      | grep -c '[0-9]' \
      | awk '{print ($1>0 ? "active" : "inactive")}'
    ```

    ## Transition Filtering

    The `MIRROR_DIFF_JSON` inside triggered jobs contains the field-level diff. Filter to only the transition direction you care about:

    ```bash
    # Only act when state changes to "active" (meeting started)
    echo "$MIRROR_DIFF_JSON" | jq -e '.[] | select(.path == "state" and .to == "active")' > /dev/null || exit 0

    # Only act when state changes to "inactive" (meeting ended)
    echo "$MIRROR_DIFF_JSON" | jq -e '.[] | select(.path == "state" and .to == "inactive")' > /dev/null || exit 0
    ```

    ## Example Jobs

    ### Meeting Started — Create a LifeOS Note

    ```bash
    # 1. Get the connector ID
    CONN_ID=$(loom connector list --json | jq -r '.[] | select(.name=="mic-state") | .id')

    # 2. Add the job
    loom add \
      --name "meeting-started-note" \
      --trigger connector \
      --connector-id $CONN_ID \
      --executor amplifier \
      --prompt "Check MIRROR_DIFF_JSON. If mic state changed to 'active', create a new meeting note in the LifeOS vault under Work/Notes with today's date and title 'Meeting <HH:MM>'."
    ```

    ### Meeting Ended — Summarize and File

    ```bash
    loom add \
      --name "meeting-ended-summarize" \
      --trigger connector \
      --connector-id $CONN_ID \
      --executor amplifier \
      --prompt "Check MIRROR_DIFF_JSON. If mic state changed to 'inactive', find the most recent meeting note created today in Work/Notes and add summary placeholder sections if missing (## Summary, ## Action Items)."
    ```

    ### macOS Do Not Disturb

    ```bash
    loom add \
      --name "meeting-dnd" \
      --trigger connector \
      --connector-id $CONN_ID \
      --executor shell \
      --command 'echo "$MIRROR_DIFF_JSON" | jq -e ".[0] | select(.path==\"state\" and .to==\"active\")" && defaults write com.apple.notificationcenterui doNotDisturb -bool true && killall -HUP usernoted 2>/dev/null || true'
    ```

    ## Sustain Window Note

    At a 4s poll interval, a brief mute during a meeting won't trigger end. For workflows where false-end triggers are costly, guard against short inactive periods:

    ```bash
    # Only act if inactive state has persisted 60+ seconds
    SINCE=$(echo "$MIRROR_CURR_JSON" | jq -r '.since')
    INACTIVE_FOR=$(( $(date +%s) - $(date -jf "%Y-%m-%dT%H:%M:%SZ" "$SINCE" "+%s" 2>/dev/null || date -d "$SINCE" +%s) ))
    [ $INACTIVE_FOR -lt 60 ] && exit 0
    ```

    ## Future: Native Event-Driven Trigger

    A future `mic` trigger type in Loom would be backed by the same platform-native binaries used in OpenWhispr — CoreAudio on macOS, WASAPI on Windows, pactl on Linux — delivering real-time transitions with no polling overhead:

    ```bash
    # Hypothetical future API
    loom add \
      --name "meeting-started" \
      --trigger mic \
      --on active \
      --sustain 2s \
      --executor amplifier \
      --prompt "Meeting started. Create a meeting note."
    ```

    The binary protocol already exists and is production-proven in OpenWhispr. Integration work: embed the binary as a Loom daemon plugin, implement a `mic` trigger type subscribing to its stdout, apply configurable sustain logic before firing jobs.

    ## See Also

    - OpenWhispr `src/helpers/audioActivityDetector.js` — reference JS implementation of the detection layer
    - OpenWhispr `resources/macos-mic-listener.swift` — CoreAudio binary source
    - Amplifier Specs: `Meeting-Aware Event Triggering` pattern — platform mechanisms, anti-patterns, and Amplifier integration shape
    