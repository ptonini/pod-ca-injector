docker_build(
    'kac',
    context = '.',
    ignore = ['demo/test_pod.yaml']
)

k8s_yaml([
    './demo/namespace.yaml',
    './demo/kac-secret.yaml',
    './demo/kac-clusterrole.yaml',
    './demo/kac-clusterrolebinding.yaml',
    './demo/kac-webhook.yaml',
    './demo/kac-service.yaml',
    './demo/kac-configmap.yaml',
    './demo/kac-serviceaccount.yaml',
    './demo/kac-deployment.yaml',
])
