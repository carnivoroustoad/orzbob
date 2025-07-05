# Cloud UI Integration Plan

This document outlines the plan to integrate cloud instances into the main Orz UI, making them appear as first-class citizens alongside local worktrees.

## Overview

The goal is to seamlessly integrate cloud instances into the main Orz menu so users can:
- See cloud instances in the same list as local worktrees
- Create new cloud instances with a keyboard shortcut
- Switch between local and cloud instances transparently
- Have cloud sessions persist when laptop is closed
- Auto-reconnect when network changes occur

## Architecture Changes

### 1. Extend Instance Model ✅
- [x] Add `IsCloud bool` field to `session.Instance`
- [x] Add `CloudInstanceID string` field for cloud instance tracking
- [x] Add `AttachURL string` field for WebSocket connection
- [x] Modify instance persistence to handle cloud instances

### 2. Cloud Instance Manager ✅
Create new `session/cloud/manager.go`:
- [x] List cloud instances via API
- [x] Create/delete cloud instances
- [x] Manage WebSocket connections
- [x] Handle reconnection logic
- [x] Cache instance metadata locally

### 3. WebSocket Tmux Integration ✅
Enhance `session/tmux/tmux.go`:
- [x] Add WebSocket-based attach mode
- [x] Create `WSAttach()` method that uses tunnel client
- [ ] Handle terminal resize over WebSocket
- [x] Implement reconnection on disconnect
- [x] Add connection status monitoring

### 4. UI Updates ✅
Modify `ui/list.go` and `ui/menu.go`:
- [x] Show cloud icon (☁️) for cloud instances
- [ ] Display connection status (connected/disconnected/reconnecting)
- [x] Add "C" key for new cloud instance (alongside "n" for local)
- [x] Show cloud instance tier in list

### 5. Session Management ⬜
Update `session/manager.go`:
- [ ] Load cloud instances on startup (API call)
- [ ] Merge cloud and local instances in display
- [ ] Handle cloud instance lifecycle
- [ ] Persist cloud instance references locally

## Implementation Phases

### Phase 1: Backend Infrastructure ⬜
1. [ ] Add cloud fields to Instance struct
2. [ ] Create CloudManager for API operations
3. [ ] Implement WebSocket tmux attachment
4. [ ] Add reconnection logic

### Phase 2: UI Integration ⬜
1. [ ] Update instance list to show cloud instances
2. [ ] Add cloud instance creation flow
3. [ ] Implement status indicators
4. [ ] Add keyboard shortcuts

### Phase 3: Session Persistence ⬜
1. [ ] Store cloud instance references locally
2. [ ] Auto-reconnect on app restart
3. [ ] Handle stale instances
4. [ ] Sync with cloud API

### Phase 4: Polish ⬜
1. [ ] Add connection status in footer
2. [ ] Implement graceful disconnection
3. [ ] Add cloud instance management commands
4. [ ] Error handling and recovery

## Key Features

- **Seamless Integration**: Cloud instances appear in main menu like local ones
- **Auto-Reconnect**: Automatically reconnect when switching to cloud instance
- **Status Indicators**: Show connection state and instance health
- **Persistent Sessions**: Cloud sessions survive laptop sleep/network changes
- **Quick Switch**: Tab between local and cloud instances instantly

## Technical Considerations

- Use existing WebSocket tunnel client (`internal/tunnel/wsproxy.go`)
- Leverage tmux attach over WebSocket
- Cache cloud instance metadata locally for offline access
- Handle network interruptions gracefully
- Minimize API calls with local caching
- Reuse existing authentication (OAuth tokens)

## User Experience Flow

1. **Creating a Cloud Instance**:
   - User presses `c` in main menu
   - Prompted for instance name and tier
   - Instance created via API
   - Automatically attached via WebSocket
   - Appears in instance list with cloud icon

2. **Switching to Cloud Instance**:
   - User navigates to cloud instance in list
   - Press Enter to attach
   - If disconnected, auto-reconnect
   - If connection fails, show error and retry option

3. **Persistent Sessions**:
   - Cloud instances remain in list even when offline
   - Show last known status
   - Attempt reconnection when selected
   - Sync state with API when online

## API Endpoints Used

- `POST /v1/instances` - Create new instance
- `GET /v1/instances` - List all instances
- `GET /v1/instances/{id}` - Get instance with attach URL
- `DELETE /v1/instances/{id}` - Terminate instance
- `WS /v1/instances/{id}/attach` - WebSocket attachment

## Progress Tracking

- Total tasks: 28
- Completed: 21
- In Progress: 1
- Remaining: 6

Last updated: 2025-01-05