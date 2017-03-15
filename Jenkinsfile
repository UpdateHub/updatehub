def GOPKG = 'code.ossystems.com.br/updatehub/agent'
def GOPATH = "/go/src/${GOPKG}"

node('docker') {
    stage('Build') {
        checkout scm
        docker.image('golang').inside("-v $WORKSPACE:/go/src/${GOPKG}") {
            sh "cd ${GOPATH} && glide --no-color install"
            sh "cd ${GOPATH} && go build -v"
            sh "cd ${GOPATH} && go install && gometalinter --deadline=30s --aggregate > lint.txt || true"
        }

        step([$class: 'WarningsPublisher',
              parserConfigurations: [
                  [parserName: 'Go Lint', pattern: 'lint.txt']
              ]])
    }

    stage('Test') {
        try {
            docker.image('golang').inside("-v $WORKSPACE:${GOPATH}") {
                sh "cd ${GOPATH} && go test -v \$(glide novendor | tr '\n' ' ') | tee test-result.log"
                sh "cd ${GOPATH} && gocov test \$(glide novendor | tr '\n' ' ') | gocov-html > coverage.html"
                sh "cd ${GOPATH} && go2xunit -fail -input test-result.log -output test-result.xml"
            }
        } finally {
            publishHTML([alwaysLinkToLastBuild: true,
                         reportDir: "$WORKSPACE",
                         reportFiles: 'coverage.html',
                         reportName: 'Coverage Report'])

            step([$class: 'XUnitBuilder',
                  thresholds: [
                      [$class: 'FailedThreshold', failureThreshold: '1']
                  ],
                  tools: [
                      [$class: 'JUnitType', pattern: 'test-result.xml']
                  ]])
        }
    }

    deleteDir()
}
