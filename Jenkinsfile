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
        
        stage('Install Terraform') {
            steps {
                sh '''
                    # Check current Terraform version
                    terraform version || echo "Terraform not found"
                    
                    # Install Terraform 1.14.1 to match local version
                    cd /tmp
                    wget -q https://releases.hashicorp.com/terraform/1.14.1/terraform_1.14.1_linux_amd64.zip
                    unzip -o terraform_1.14.1_linux_amd64.zip
                    sudo mv terraform /usr/local/bin/
                    rm terraform_1.14.1_linux_amd64.zip
                    
                    # Verify installation
                    terraform version
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
                        # Use the admin public key from the workspace
                        terrasign wrap --key admin.pub -- apply tfplan
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
