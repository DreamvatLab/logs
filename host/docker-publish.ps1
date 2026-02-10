# !!!!! Make sure ./build.ps1 already executed without problem first

$imageName = "logs"
$version = "1.0.6"
$targetDir = "./dist"

Write-Host "#: loading docker image"
docker load -i $targetDir/$imageName.tar
docker tag $imageName dreamvat/$imageName`:latest
docker tag $imageName dreamvat/$imageName`:$version
Write-Host "#: pushing docker image with tags: latest, $version"
docker push dreamvat/$imageName`:latest
docker push dreamvat/$imageName`:$version
Write-Host "#: clear temperary files..."
docker rmi dreamvat/$imageName`:latest
docker rmi dreamvat/$imageName`:$version
docker rmi $imageName
Write-Host "#: done"