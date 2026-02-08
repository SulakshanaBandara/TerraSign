pipeline {
    agent any
    
    environment {
        TF_IN_AUTOMATION = 'true'
        TERRASIGN_SERVICE = credentials('terrasign-service-url')  // http://terrasign-server:8081
        COSIGN_PASSWORD = credentials('cosign-password')          // Empty for demo keys
        ADMIN_PUBLIC_KEY = credentials('admin-public-key-path')   // Path to admin.pub
    }
    
    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }
        
        stage('Build TerraSign') {
            steps {
                sh '''
                    # Build terrasign from source
                    cd cmd/terrasign
                    go build -o $HOME/go/bin/terrasign .
                    
                    # Verify build (just check if binary exists and is executable)
                    ls -lh $HOME/go/bin/terrasign
                    echo "TerraSign binary built successfully"
                '''
            }
        }
        
        stage('Terraform Init') {
            steps {
                dir('examples/simple-app') {
                    sh 'terraform init'
                }
            }
        }
        
        stage('Terraform Plan') {
            steps {
                dir('examples/simple-app') {
                    sh 'terraform plan -out=tfplan'
                }
            }
        }
        
        stage('Start TerraSign Server') {
            steps {
                script {
                    // Start TerraSign server in background
                    sh '''
                        export PATH=$PATH:$HOME/go/bin
                        
                        # Kill any existing server on port 8081
                        pkill -f "terrasign server" || true
                        
                        # Start server in background
                        nohup terrasign server --port 8081 --storage ./demo-storage > terrasign-server.log 2>&1 &
                        
                        # Wait for server to be ready
                        echo "Waiting for TerraSign server to start..."
                        attempt=1
                        max_attempts=30
                        while [ $attempt -le $max_attempts ]; do
                            if curl -s http://localhost:8081/list-pending > /dev/null 2>&1; then
                                echo "TerraSign server is ready!"
                                exit 0
                            fi
                            echo "Attempt $attempt/$max_attempts: Server not ready yet..."
                            sleep 1
                            attempt=$((attempt + 1))
                        done
                        
                        echo "ERROR: Server failed to start"
                        cat terrasign-server.log
                        exit 1
                    '''
                }
            }
        }
        
        stage('Submit for Review') {
            steps {
                dir('examples/simple-app') {
                    script {
                        // Submit plan to signing service
                        def output = sh(
                            script: """
                                export PATH=\$PATH:\$HOME/go/bin
                                terrasign submit-for-review --service ${TERRASIGN_SERVICE} tfplan
                            """,
                            returnStdout: true
                        ).trim()
                        
                        // Extract submission ID
                        def submissionId = (output =~ /Submission ID: ([a-f0-9-]+)/)[0][1]
                        env.PLAN_ID = submissionId
                        
                        echo "Plan submitted with ID: ${submissionId}"
                        echo "Waiting for admin approval..."
                    }
                }
            }
        }
        
        stage('Wait for Approval') {
            steps {
                // Pause pipeline and wait for manual admin approval
                input message: "Plan ${env.PLAN_ID} submitted. Admin must sign before proceeding.",
                      ok: 'Plan Signed - Continue'
            }
        }
        
        stage('Download Signature') {
            steps {
                dir('examples/simple-app') {
                    sh """
                        curl -o tfplan.sig ${TERRASIGN_SERVICE}/download/${env.PLAN_ID}/signature
                    """
                }
            }
        }
        
        stage('Verify and Apply') {
            steps {
                dir('examples/simple-app') {
                    // Use terrasign wrapper to verify before applying
                    sh """
                        export PATH=\$PATH:\$HOME/go/bin
                        terrasign wrap --key ${ADMIN_PUBLIC_KEY} -- apply tfplan
                    """
                }
            }
        }
    }
    
    post {
        always {
            cleanWs()
        }
    }
}
