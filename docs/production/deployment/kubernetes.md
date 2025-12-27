# Kubernetes Deployment

Deploy Conductor on Kubernetes for scalable, multi-node production environments.

## Namespace

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: conductor
```

```bash
kubectl apply -f namespace.yaml
```

## Secrets

Store API keys as Kubernetes secrets:

```bash
kubectl create secret generic conductor-secrets \
  --from-literal=anthropic-api-key="${ANTHROPIC_API_KEY}" \
  --namespace conductor
```

:::caution[Secret Security]
Never commit secrets to version control. Use sealed secrets, external secrets operators, or inject at deployment time.
:::

## Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: conductor
  namespace: conductor
spec:
  replicas: 2
  selector:
    matchLabels:
      app: conductor
  template:
    metadata:
      labels:
        app: conductor
    spec:
      containers:
      - name: conductor
        image: ghcr.io/tombee/conductor:latest
        ports:
        - containerPort: 9000
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: conductor-secrets
              key: anthropic-api-key
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 9000
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 9000
          initialDelaySeconds: 5
          periodSeconds: 10
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: conductor-data
```

## Persistent Volume

```yaml
# pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: conductor-data
  namespace: conductor
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
```

## Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: conductor
  namespace: conductor
spec:
  type: ClusterIP
  ports:
  - port: 9000
    targetPort: 9000
  selector:
    app: conductor
```

## Ingress (Optional)

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: conductor
  namespace: conductor
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - conductor.example.com
    secretName: conductor-tls
  rules:
  - host: conductor.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: conductor
            port:
              number: 9000
```

## Deploy All

```bash
kubectl apply -f namespace.yaml
kubectl apply -f pvc.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml
```

Verify:

```bash
kubectl get pods -n conductor
kubectl logs -n conductor -l app=conductor -f
```

## Horizontal Pod Autoscaler

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: conductor
  namespace: conductor
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: conductor
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```
