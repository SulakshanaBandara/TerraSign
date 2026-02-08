pipeline {
    agent any
    
    environment {
        TF_IN_AUTOMATION = 'true'
        TERRASIGN_SERVICE = credentials('terrasign-service-url')  // http://terrasign-server:8081
        COSIGN_PASSWORD = credentials('cosign-password')          // Empty for demo keys
        ADMIN_PUBLIC_KEY = credentials('admin-public-key-path')   // Path to admin.pub
    }
    
    stages {
        stage('Install TerraSign') {
            steps {
                sh '''
                    # Install Go if not available
                    which go || (echo "Go not installed" && exit 1)
                    
                    # Install terrasign
                    go install github.com/sulakshanakarunarathne/terrasign/cmd/terrasign@latest
                    
                    # Add to PATH for this session
                    export PATH=$PATH:$HOME/go/bin
                    
                    # Verify installation
                    terrasign --help || echo "TerraSign installed but not in PATH"
                '''
            }
        }
        
        stage('Checkout') {
            steps {
                checkout scm
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
                        terrasign wrap --key ${ADMIN_PUBLIC_KEY} -- apply tfplan
                    """
                }
            }
        }
    }
    
    post {
        success {
            echo 'Deployment successful with verified signature!'
        }
        failure {
            echo 'Deployment failed - check verification logs'
        }
        always {
            cleanWs()
        }
    }
}
