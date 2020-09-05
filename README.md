# job-generator

* local run:
```
docker run -e NAMESPACE=<NAMESPACE> -e scmProvider=<scmProvider> -e HELM_RELEASE=<HELM_RELEASE> agnops/job-generator
```

TODO:
1. Handle the generated yaml nodeSelector as apart from installation
2. Embed /data/deploymentEnvs if isDeployment
3. Add checkout toggle
4. Add ramDisk feature to external dir
5. Replace OAUTH_TOKEN with integrated function
6. Checkout only registered repositories from redis