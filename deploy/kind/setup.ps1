$ErrorActionPreference = "Stop"

$KIND = "kind"
$KUBECTL = "kubectl"
$SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Path

function Write-Step($msg) {
    Write-Host "`n==> $msg" -ForegroundColor Cyan
}

function Write-Info($msg) {
    Write-Host "    $msg" -ForegroundColor Gray
}

function Check-Prerequisites {
    Write-Step "Checking prerequisites..."
    $missing = @()
    foreach ($tool in @($KIND, $KUBECTL, "docker")) {
        try {
            Get-Command $tool -ErrorAction Stop | Out-Null
            Write-Info "$tool found"
        } catch {
            $missing += $tool
        }
    }
    if ($missing.Count -gt 0) {
        Write-Host "ERROR: Missing tools: $($missing -join ', ')" -ForegroundColor Red
        Write-Host "Install them before running this script." -ForegroundColor Red
        exit 1
    }
}

function Create-Cluster($configFile, $clusterName) {
    Write-Step "Creating cluster: $clusterName"
    $configPath = Join-Path $SCRIPT_DIR $configFile
    if (-not (Test-Path $configPath)) {
        Write-Host "ERROR: Config file not found: $configPath" -ForegroundColor Red
        exit 1
    }
    & $KIND create cluster --config $configPath --name $clusterName --wait 120s
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: Failed to create cluster $clusterName" -ForegroundColor Red
        exit 1
    }
    Write-Info "Cluster $clusterName created successfully"
}

function Setup-NetworkPolicy($clusterName) {
    Write-Step "Applying network policies to $clusterName"
    & $KUBECTL config use-context "kind-$clusterName"
    $policyPath = Join-Path $SCRIPT_DIR "network-policy.yaml"
    & $KUBECTL apply -f $policyPath
    Write-Info "Network policies applied to $clusterName"
}

function Label-Namespace($clusterName) {
    Write-Step "Labeling namespaces in $clusterName"
    & $KUBECTL config use-context "kind-$clusterName"
    & $KUBECTL label namespace obs-platform name=obs-platform --overwrite
    & $KUBECTL create namespace monitoring 2>$null
    & $KUBECTL label namespace monitoring name=monitoring --overwrite
    Write-Info "Namespaces labeled in $clusterName"
}

function Setup-InterClusterConnectivity {
    Write-Step "Setting up inter-cluster connectivity"
    Write-Info "Cluster 1 context: kind-obs-cluster-1 (podSubnet: 10.10.0.0/16)"
    Write-Info "Cluster 2 context: kind-obs-cluster-2 (podSubnet: 10.20.0.0/16)"
    Write-Info ""
    Write-Info "To enable cross-cluster communication, use one of:"
    Write-Info "  1. Submariner (https://submariner.io/)"
    Write-Info "  2. Skupper (https://skupper.io/)"
    Write-Info "  3. Istio multi-cluster (https://istio.io/)"
    Write-Info ""
    Write-Info "For testing, you can use kubectl port-forward and service mesh"
}

function Deploy-ObsPlatform($clusterName) {
    Write-Step "Deploying obs-platform to $clusterName"
    & $KUBECTL config use-context "kind-$clusterName"
    & $KUBECTL apply -f $SCRIPT_DIR\..\..\deploy\compose\docker-compose.yml 2>$null
    Write-Info "Platform deployment initiated on $clusterName"
}

function Main {
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "  OBS Platform K8S Test Environment    " -ForegroundColor Green
    Write-Host "  Multi-Cluster Setup Script           " -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green

    Check-Prerequisites

    $action = if ($args.Count -gt 0) { $args[0] } else { "create" }

    switch ($action) {
        "create" {
            Create-Cluster "cluster1.yaml" "obs-cluster-1"
            Create-Cluster "cluster2.yaml" "obs-cluster-2"
            Setup-NetworkPolicy "obs-cluster-1"
            Setup-NetworkPolicy "obs-cluster-2"
            Label-Namespace "obs-cluster-1"
            Label-Namespace "obs-cluster-2"
            Setup-InterClusterConnectivity
            Write-Step "Setup complete!"
            Write-Host ""
            Write-Host "Clusters:" -ForegroundColor Yellow
            & $KIND get clusters
            Write-Host ""
            Write-Host "Current context:" -ForegroundColor Yellow
            & $KUBECTL config current-context
        }
        "destroy" {
            Write-Step "Destroying clusters..."
            & $KIND delete cluster --name obs-cluster-1
            & $KIND delete cluster --name obs-cluster-2
            Write-Step "Clusters destroyed"
        }
        "status" {
            Write-Step "Cluster status:"
            & $KIND get clusters
            foreach ($cluster in @("obs-cluster-1", "obs-cluster-2")) {
                Write-Host "`n--- $cluster ---" -ForegroundColor Yellow
                & $KUBECTL config use-context "kind-$cluster" 2>$null
                & $KUBECTL get nodes
                & $KUBECTL get namespaces
            }
        }
        default {
            Write-Host "Usage: .\setup.ps1 [create|destroy|status]" -ForegroundColor Yellow
            Write-Host "  create  - Create multi-cluster environment (default)"
            Write-Host "  destroy - Destroy all clusters"
            Write-Host "  status  - Show cluster status"
        }
    }
}

Main @args
