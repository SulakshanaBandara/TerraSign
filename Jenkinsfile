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


        stage('Verify Commit Signatures') {
            steps {
                script {
                    // Verify the latest commit is signed
                    def result = sh(
                        script: 'git verify-commit HEAD 2>&1 || echo "UNSIGNED"',
                        returnStdout: true
                    ).trim()
                    
                    if (result.contains('UNSIGNED') || result.contains('BAD signature')) {
                        echo """
                        ========================================
                        WARNING: Commit is not properly signed!
                        ========================================
                        
                        For production deployments, all commits must be GPG signed.
                        See docs/commit_signing_guide.md for setup instructions.
                        
                        Continuing for demo purposes...
                        """
                        // In production, you would fail here:
                        // error('Unsigned commit detected!')
                    } else {
                        echo '✓ Commit signature verified'
                    }
                }
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
        
        stage('Discover Service') {
            steps {
                script {
                    def candidateUrls = [
                        env.TERRASIGN_SERVICE,
                        "http://host.docker.internal:8081",
                        "http://172.17.0.1:8081",
                        "http://localhost:8081"
                    ]
                    
                    env.REACHABLE_SERVICE = ""
                    for (url in candidateUrls) {
                        try {
                            echo "Testing connection to ${url}..."
                            // Try calling list-pending which should return 200 or 403 (if lockdown active)
                            def status = sh(script: "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 2 ${url}/list-pending || true", returnStdout: true).trim()
                            if (status == '200' || status == '403') {
                                env.REACHABLE_SERVICE = url
                                echo "Success! Reached server at ${url}"
                                break
                            }
                        } catch (Exception e) {
                            echo "Failed: ${e.getMessage()}"
                        }
                    }
                    
                    if (env.REACHABLE_SERVICE == "") {
                        error("Could not reach TerraSign service on any candidate URLs. Ensure the server is running on the host machine.")
                    }
                }
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
                                terrasign submit-for-review --service ${REACHABLE_SERVICE} tfplan
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
        
        stage('Download Signature Bundle') {
            steps {
                dir('examples/simple-app') {
                    sh """
                        # Download the full cosign bundle which contains the ECDSA signature
                        curl -sSL -o tfplan.bundle ${REACHABLE_SERVICE}/download/${env.PLAN_ID}/bundle || true
                        
                        # Also download legacy signature as fallback
                        curl -sSL -o tfplan.sig ${REACHABLE_SERVICE}/download/${env.PLAN_ID}/signature || true
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
                        # Use the admin public key injected via Jenkins credential
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
