docker_build(
    'pod-ca-injector',
    context = '.',
    ignore = ['demo/test_pod.yaml']
)

k8s_yaml([
    './demo/namespace.yaml',
    './demo/injector-secret.yaml',
    './demo/injector-clusterrole.yaml',
    './demo/injector-clusterrolebinding.yaml',
    './demo/injector-webhook.yaml',
    './demo/injector-service.yaml',
    './demo/injector-configmap.yaml',
    './demo/injector-serviceaccount.yaml',
    './demo/injector-deployment.yaml',
])
