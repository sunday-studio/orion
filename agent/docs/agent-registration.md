## 🧩 Agent Registration Flow (Automatic)

### Overview

Agent registration in Orion is fully automated.  
When a new agent binary starts up, it will automatically register itself with the Orion Core server if it doesn’t already have an identity.

This ensures zero manual setup — simply install and start the agent binary, and it becomes part of the Orion network.

---

### Step-by-Step Flow

1. **Startup Check**
   - On startup, the agent checks its local config file (e.g., `/etc/orion/config.yaml` or `~/.orion/config.json`).
   - If it finds `agent_id` and `token`, registration is skipped — it proceeds to normal operation.

2. **UUID Generation**
   - If no config exists, the agent generates a **unique UUID** to identify itself.
   - UUID generation combines system info such as:
     - Hostname
     - MAC address
   - The resulting UUID is stored in the config file so it remains consistent across restarts.

3. **Registration Request**
   - The agent sends a `POST /register` request to the Orion Core with its details:
     ```json
     {
       "uuid": "<generated-uuid>",
       "name": "<hostname>",
       "os": "<linux|macos>",
       "arch": "<amd64|arm64>"
     }
     ```
    after the registration is complete; the server will return 
    ```json
        {
      "success": true,
      "message": "Agent registered successfully",
      "data": {
        "agent_id": 1,
        "token": "permanent-authentication-token"
      }
    }
    ```
    save the token in the config and attach it as a header when making report calls

4. **Core Server Handling**
   - Orion Core checks if an agent with the same UUID already exists:
     - ✅ **Exists:** returns the existing agent ID and token.
     - ❌ **Doesn’t exist:** creates a new record and issues a new permanent token.

5. **Response**
   - Core responds with:
     ```json
     {
       "agent_id": 42,
       "token": "f9c23a45b7..."
     }
     ```
   - The agent saves this in its local config file:
     ```
     agent_id: 42
     token: f9c23a45b7...
     ```

6. **Subsequent Operations**
   - All future network calls (e.g., `/report/:agent_id`) include the token via an `Authorization: Bearer` header.
   - The token never expires.
   - The Core uses it to validate that the request belongs to the registered agent.

---

### Security Considerations

- No manual key exchange needed for now.
- Registration endpoint can be rate-limited or IP-restricted if deployed publicly.
- Later, an optional **registration secret** can be introduced for controlled environments.

---

### Benefits

✅ Fully automated onboarding — ideal for self-installing agents.  
✅ UUID ensures unique identity per machine.  
✅ Resilient — if the Core restarts or data is reset, agents can safely re-register.  
✅ Simple for initial development and local testing.

---

### Future Enhancements

- Add an optional `registration_secret` parameter for environments that require controlled registration.
- Allow admin-driven pre-registration through the UI or CLI.
- Add `last_seen` and `version` tracking for better agent lifecycle management.
