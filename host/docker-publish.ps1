# !!!!! Make sure ./build.ps1 already executed without problem first

$imageName = "logs"
$targetDir = "./dist"

Write-Host "#: loading docker image"
docker load -i $targetDir/$imageName.tar
docker tag $imageName dreamvat/$imageName
Write-Host "#: pushing docker image"
docker push dreamvat/$imageName
Write-Host "#: clear temperary files..."
docker rmi dreamvat/$imageName
docker rmi $imageName
Write-Host "#: done"