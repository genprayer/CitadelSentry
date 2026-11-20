# Technical Feasibility Report: Native Server Browser Integration

## 1. Executive Summary
This report evaluates the feasibility of achieving seamless server joining in CitadelSeeder through native Squad 10.6.3 server browser integration. Current approaches utilizing synthetic URIs violate established architectural constraints (ADR 0001). This document outlines a path toward legitimate integration via configuration manipulation to ensure discoverability within the native game engine's browser, aligning with the project's long-term stability and security objectives.

## 2. Audit Results (Website vs Sentry)
*   **Citadel-WebProject**: Initial findings suggest that success attributed to web-based connection logic is misattributed. The observed connectivity is a result of legitimate game browser features—specifically the browser recognizing sessions through native discovery mechanisms—rather than direct interaction initiated by the web project.
*   **CitadelSeeder (Sentry)**: The current implementation of `JoinServerDirect` has been identified as a violation of ADR 0001. It relies on synthetic `steam://` URIs, which are ineffective for direct joining and contrary to the established architectural standards for this project.

## 3. Binary Analysis and R3 Requirement Technical Deep-Dive
Technical investigation confirms that native Squad 10.6+ joining flows are strictly controlled within the engine. Direct function calls to trigger travel are protected and non-exposed in standard shipping binaries.

### R3.1 State Requirements for `QueueForServer` / `ClientTravel`
Successful execution of `USQGameInstance::QueueForServer` requires a valid, pre-populated `FSQServerItem` object. This object holds essential metadata (server address, port, Beacon port, EOS session mapping, map data). The client engine will reject any attempt to travel if the internal `FSQServerItem` cache does not contain a verified entry for the target endpoint. Simply providing an IP/Port is insufficient; the client's internal browser cache MUST be pre-populated.

### R3.2 Party State, EOS Sessions, and `JoinBeacon` Lifecycle
The `USQGameInstance` manages the connection flow via the `ASQJoinBeaconClient`. The sequence is as follows:
1.  **Handshake**: Upon `JoinSession`, the game instantiates an `ASQJoinBeaconClient`, which attempts a connection to the target server's Beacon port (15005).
2.  **Authentication/Verification**: During this handshake, the client passes the current Party/EOS authentication tokens to verify the user is allowed to join the target session.
3.  **State Synchronization**: If authentication succeeds, the Beacon notifies `USQGameInstance`, which proceeds to trigger `ClientTravel` via the `GameNetDriver` on the Game port (7787).
4.  **Failure Scenario**: If the `FSQServerItem` was improperly generated or if the EOS session ID does not match the active Beacon's expected token, `JoinSession` will fail, and `ClientTravel` will not occur.

### R3.3 Legitimate Desktop Application Influence (ADR 0001 Compliant)
A desktop application cannot *inject* this state directly into the engine's memory due to security protections. However, it can *influence* the state by manipulating persistent game configuration files:
*   **Mechanism**: The application must modify `GameUserSettings.ini` to inject the target server's IP and port into the user's "Favorites" or recent connection cache.
*   **Outcome**: This mechanism forces the game engine to populate the `FSQServerItem` cache upon the next client launch (or server list refresh), rendering the target server discoverable by the native `QueueForServer` pipeline without synthetic URI violations.


## 4. Feasibility Analysis & ADR 0001 Compliance
The synthetic approach previously attempted in CitadelSeeder is fundamentally flawed and prohibited.

*   **ADR 0001 Compliance**: All connection attempts must eschew synthetic `steam://` URIs, console command injections, and keyboard emulation.
*   **Conclusion**: Any effort to bypass the native `QueueForServer` workflow via simulated inputs is non-viable and architecturally rejected.

## 5. Proposed Legitimate Architectural Route (Configuration Manipulation)
The only viable path forward is to programmatically manipulate the game's local configuration, specifically `GameUserSettings.ini`.

*   **Objective**: Ensure the target Citadel server is persistently listed in the user's "Favorites" or ensure that "Hide Empty Servers" filters do not exclude the target server.
*   **Mechanism**:
    1.  Access the local `GameUserSettings.ini` file.
    2.  Inject/update the target server's IP and port into the list of favorite servers.
    3.  Modify filter settings to ensure the server is not hidden due to player counts.
*   **Outcome**: The server becomes natively discoverable in the Squad Server Browser, allowing the user to initiate the `JoinSession` flow through legitimate UI channels.

## 6. Recommendations & Next Steps
1.  **Immediate Remediation**: Remove all code related to prohibited synthetic `steam://` URI connection attempts.
2.  **Implementation**: Develop a robust configuration manipulation utility to handle `GameUserSettings.ini` parsing and updates.
3.  **Validation**: Test the discovery mechanism across multiple Squad 10.6.3 installations to ensure configuration changes propagate correctly.
4.  **UI/UX**: Update the Citadel Sentry UI to communicate clearly that the server is being made discoverable in the "Favorites" tab.
