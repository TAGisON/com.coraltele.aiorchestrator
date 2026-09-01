# Install, publish, and smoke-test the Coral TFN desk against a running lab.
param(
    [string]$Base = "http://127.0.0.1:8012"
)

$ErrorActionPreference = "Stop"

function Invoke-Api($Method, $Path, $Body) {
    $params = @{
        Uri             = "$Base$Path"
        Method          = $Method
        UseBasicParsing = $true
    }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = ($Body | ConvertTo-Json -Depth 20 -Compress)
    }
    return Invoke-RestMethod @params
}

Write-Host "=== engines ==="
try {
    $eng = Invoke-Api GET "/v1/tenant/engines"
    Write-Host ("already set listen={0} think={1} speak={2}" -f $eng.listen, $eng.think, $eng.speak)
} catch {
    Invoke-Api PUT "/v1/tenant/engines" @{ listen = "fake-listen"; think = "fake-think"; speak = "fake-speak" } | ConvertTo-Json -Compress
    Write-Host "saved fake engines for offline lab"
}
Write-Host ""

Write-Host "=== install coral-tfn ==="
$inst = Invoke-Api POST "/v1/desk-presets/coral-tfn" @{ tenant_id = "default" }
Write-Host ("desk={0} publishable={1}" -f $inst.desk.id, $inst.checklist.publishable)

Write-Host "=== publish ==="
$pub = Invoke-Api POST "/v1/desks/coral-tfn/publish" @{ published_by = "setup-script" }
Write-Host ("desk_version={0} profile={1} v{2}" -f $pub.desk_version, $pub.profile_id, $pub.profile_version)

function Show-Sim($Title, $Lang, $Turns) {
    Write-Host ""
    Write-Host "=== $Title ($Lang) ==="
    $sim = Invoke-Api POST "/v1/desks/coral-tfn/simulate" @{
        language = $Lang
        turns    = $Turns
    }
    foreach ($s in $sim.steps) {
        if ($s.user) { Write-Host ("CALLER [{0}]: {1}" -f $s.language, $s.user) }
        Write-Host ("AI     [{0}]: {1}" -f $s.language, $s.assistant)
    }
    Write-Host ("ended={0} disposition={1} ticket={2}" -f $sim.ended, $sim.disposition, $sim.attributes.ticket_id)
    return $sim
}

Show-Sim "sales EN" "en-IN" @("I want a quotation for IP phones", "20 phones for our Delhi office") | Out-Null
Show-Sim "complaint HI" "hi-IN" @(
    "मुझे शिकायत दर्ज करानी है",
    "आईपी फोन",
    "हाँ चालू है",
    "नेटवर्क है",
    "कॉल नहीं लग रही",
    "कई फोन",
    "शिकायत दर्ज करें",
    "सुरेश शर्मा",
    "suresh@coral.com",
    "हाँ सही है",
    "हाँ बिल्कुल सही",
    "नहीं धन्यवाद"
) | Out-Null

Write-Host ""
Write-Host "Ready: $Base/admin/  $Base/user/  $Base/supervisor/"
