# Reusable Chrome DevTools Protocol (CDP) helpers for driving the GitHub web UI
# through the human's debug Chrome session (http://127.0.0.1:9222).
#
# Dot-source this file from an operator script:
#   . "$PSScriptRoot\CdpCommon.ps1"
#
# CRITICAL PowerShell gotcha (the reason for every `$null =` below): the async
# WebSocket calls `ConnectAsync(...).GetAwaiter().GetResult()` and
# `SendAsync(...).GetAwaiter().GetResult()` return void, but in PowerShell a void
# method call that is NOT suppressed leaks an item into the enclosing function's
# output stream. That turns a function's intended single return (the socket) into
# an array, so `$ws` silently becomes `object[]` and every later `$ws.SendAsync`
# throws. Suppress every void async call with `$null =` and return the socket
# with the unary array operator (`return ,$ws`) so a single-element pipeline does
# not get unwrapped/rewrapped.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Get-OptionalPropertyValue {
    param(
        [object]$InputObject,
        [string]$Name
    )

    if ($null -eq $InputObject) {
        return $null
    }

    $property = $InputObject.PSObject.Properties[$Name]
    if ($property) {
        return $property.Value
    }

    return $null
}

function Get-CdpTargets {
    # GET /json/list and return only the page-type tabs.
    param([string]$Endpoint = 'http://127.0.0.1:9222')

    try {
        $targets = Invoke-RestMethod -Uri "$Endpoint/json/list" -TimeoutSec 5
    } catch {
        throw "Unable to reach the Chrome debug endpoint at '$Endpoint'. Start the debug Chrome profile with C:\Agent\Orchestrator\Scripts\Start-ChromeDebugProfile.ps1 (port 9222) and log into GitHub first."
    }

    return @($targets | Where-Object { $_.type -eq 'page' })
}

function Select-CdpTarget {
    # Pick exactly one page target. -TargetId selects by exact id; otherwise the
    # first -UrlContains substring match is required to be unique so we never
    # operate on the wrong GitHub tab when several are open.
    param(
        [object[]]$Targets,
        [string]$TargetId,
        [string]$UrlContains
    )

    if ($TargetId) {
        $byId = @($Targets | Where-Object { $_.id -eq $TargetId })
        if ($byId.Count -eq 0) {
            throw "No open page target matched TargetId '$TargetId'."
        }

        return $byId[0]
    }

    if (-not $UrlContains) {
        throw 'Select-CdpTarget requires either -TargetId or -UrlContains.'
    }

    $matchesUrl = @($Targets | Where-Object { $_.url -like "*$UrlContains*" })
    if ($matchesUrl.Count -eq 0) {
        return $null
    }

    if ($matchesUrl.Count -gt 1) {
        $summary = $matchesUrl | ForEach-Object { "- $($_.id) | $($_.title) | $($_.url)" }
        throw "Multiple open tabs matched '$UrlContains'. Re-run with -TargetId to disambiguate.`n$($summary -join [Environment]::NewLine)"
    }

    return $matchesUrl[0]
}

function New-CdpSocket {
    # Connect a ClientWebSocket to a tab's webSocketDebuggerUrl. See the header
    # note: the void ConnectAsync result is suppressed and the socket is returned
    # with the unary array operator so callers reliably get the socket object.
    param([Parameter(Mandatory = $true)][string]$WebSocketUrl)

    $ws = [System.Net.WebSockets.ClientWebSocket]::new()
    $null = $ws.ConnectAsync([uri]$WebSocketUrl, [System.Threading.CancellationToken]::None).GetAwaiter().GetResult()
    return , $ws
}

function Receive-CdpMessageText {
    # Receive a single complete CDP message with a per-read timeout so a missing
    # response can never hang the operator script indefinitely.
    param(
        [Parameter(Mandatory = $true)][System.Net.WebSockets.ClientWebSocket]$Socket,
        [int]$TimeoutSeconds = 15
    )

    $buffer = New-Object byte[] 65536
    $stream = [System.IO.MemoryStream]::new()
    $cts = [System.Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds($TimeoutSeconds))

    try {
        while ($true) {
            $segment = [System.ArraySegment[byte]]::new($buffer)
            $result = $Socket.ReceiveAsync($segment, $cts.Token).GetAwaiter().GetResult()

            if ($result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
                throw 'The CDP websocket was closed before a response was received.'
            }

            if ($result.Count -gt 0) {
                [void]$stream.Write($buffer, 0, $result.Count)
            }

            if ($result.EndOfMessage) {
                break
            }
        }
    } catch [System.OperationCanceledException] {
        throw "Timed out after $TimeoutSeconds seconds waiting for a CDP message."
    } finally {
        $cts.Dispose()
    }

    return [System.Text.Encoding]::UTF8.GetString($stream.ToArray())
}

function Invoke-CdpCommand {
    # Send {id,method,params} and loop receiving until the matching id arrives,
    # skipping unrelated CDP events. Returns the command result object.
    param(
        [Parameter(Mandatory = $true)][System.Net.WebSockets.ClientWebSocket]$Socket,
        [Parameter(Mandatory = $true)][int]$Id,
        [Parameter(Mandatory = $true)][string]$Method,
        [hashtable]$Params = @{},
        [int]$TimeoutSeconds = 15
    )

    $payload = @{
        id     = $Id
        method = $Method
        params = $Params
    } | ConvertTo-Json -Depth 20 -Compress

    $bytes = [System.Text.Encoding]::UTF8.GetBytes($payload)
    $segment = [System.ArraySegment[byte]]::new($bytes)
    $null = $Socket.SendAsync($segment, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, [System.Threading.CancellationToken]::None).GetAwaiter().GetResult()

    while ($true) {
        $messageText = Receive-CdpMessageText -Socket $Socket -TimeoutSeconds $TimeoutSeconds
        $message = $messageText | ConvertFrom-Json

        $messageId = Get-OptionalPropertyValue -InputObject $message -Name 'id'
        if ($messageId -ne $Id) {
            continue
        }

        $messageError = Get-OptionalPropertyValue -InputObject $message -Name 'error'
        if ($messageError) {
            throw "CDP command '$Method' failed: $($messageError.message)"
        }

        return (Get-OptionalPropertyValue -InputObject $message -Name 'result')
    }
}

function Invoke-CdpEval {
    # Runtime.evaluate with returnByValue + awaitPromise. Returns the unwrapped JS
    # value (so an async expression's resolved value comes straight back). Throws
    # with the JS exception text on a thrown error.
    param(
        [Parameter(Mandatory = $true)][System.Net.WebSockets.ClientWebSocket]$Socket,
        [Parameter(Mandatory = $true)][int]$Id,
        [Parameter(Mandatory = $true)][string]$Expression,
        [int]$TimeoutSeconds = 15
    )

    $result = Invoke-CdpCommand -Socket $Socket -Id $Id -Method 'Runtime.evaluate' -Params @{
        expression    = $Expression
        returnByValue = $true
        awaitPromise  = $true
    } -TimeoutSeconds $TimeoutSeconds

    $exceptionDetails = Get-OptionalPropertyValue -InputObject $result -Name 'exceptionDetails'
    if ($exceptionDetails) {
        $exception = Get-OptionalPropertyValue -InputObject $exceptionDetails -Name 'exception'
        $description = Get-OptionalPropertyValue -InputObject $exception -Name 'description'
        if (-not $description) {
            $description = Get-OptionalPropertyValue -InputObject $exceptionDetails -Name 'text'
        }
        throw "Runtime.evaluate threw: $description"
    }

    $remoteResult = Get-OptionalPropertyValue -InputObject $result -Name 'result'
    return (Get-OptionalPropertyValue -InputObject $remoteResult -Name 'value')
}

function Invoke-CdpClick {
    # Convenience: click the first element matching a CSS selector via JS. Returns
    # $true when an element was found and clicked, $false otherwise.
    param(
        [Parameter(Mandatory = $true)][System.Net.WebSockets.ClientWebSocket]$Socket,
        [Parameter(Mandatory = $true)][int]$Id,
        [Parameter(Mandatory = $true)][string]$Selector,
        [int]$TimeoutSeconds = 15
    )

    $escaped = $Selector.Replace('\', '\\').Replace("'", "\'")
    $expression = "(() => { const el = document.querySelector('$escaped'); if (!el) { return false; } el.click(); return true; })()"
    return [bool](Invoke-CdpEval -Socket $Socket -Id $Id -Expression $expression -TimeoutSeconds $TimeoutSeconds)
}

function Send-CdpKey {
    # Dispatch a keyDown+keyUp pair through Input.dispatchKeyEvent. Defaults to
    # Escape, which commits/closes the GitHub single-select field picker.
    param(
        [Parameter(Mandatory = $true)][System.Net.WebSockets.ClientWebSocket]$Socket,
        [Parameter(Mandatory = $true)][int]$Id,
        [string]$Key = 'Escape',
        [string]$Code = 'Escape',
        [int]$WindowsVirtualKeyCode = 27,
        [int]$TimeoutSeconds = 15
    )

    $null = Invoke-CdpCommand -Socket $Socket -Id $Id -Method 'Input.dispatchKeyEvent' -Params @{
        type                  = 'keyDown'
        key                   = $Key
        code                  = $Code
        windowsVirtualKeyCode = $WindowsVirtualKeyCode
        nativeVirtualKeyCode  = $WindowsVirtualKeyCode
    } -TimeoutSeconds $TimeoutSeconds

    $null = Invoke-CdpCommand -Socket $Socket -Id ($Id + 1) -Method 'Input.dispatchKeyEvent' -Params @{
        type                  = 'keyUp'
        key                   = $Key
        code                  = $Code
        windowsVirtualKeyCode = $WindowsVirtualKeyCode
        nativeVirtualKeyCode  = $WindowsVirtualKeyCode
    } -TimeoutSeconds $TimeoutSeconds
}

function Close-CdpSocket {
    # Best-effort graceful close + dispose. Never throws.
    param([System.Net.WebSockets.ClientWebSocket]$Socket)

    if (-not $Socket) {
        return
    }

    try {
        $null = $Socket.CloseAsync(
            [System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure,
            'done',
            [System.Threading.CancellationToken]::None
        ).GetAwaiter().GetResult()
    } catch {
    }

    $Socket.Dispose()
}
