
package main

import (
  "log"
  "fmt"
  "io"
  "os/exec"
  "syscall"
  "runtime"
  "flag"
  "unicode"
  "strings"
  "go/parser"
  "go/token"
)

const (
  MSVC_PATH_KEY string = "installationPath"
  MSVC_PATH_KEY_LENGTH int = 18
  VCVARS_PATH string = `\VC\Auxiliary\Build\`
  CMD_MAX_OUTPUT_LENGTH int = 4096
  EXPORT_KEYWORD = "//export "
  EXPORT_KEYWORD_LENGHT = 9
)

type files []string

func (f *files) String() string {
  return fmt.Sprint(*f)
}

func (f *files) Set(value string) error {
  delim := func(c rune) bool {
    return !(c != ',' && !unicode.IsSpace(c) && c != ';')
  }

  //Allow flags to be set multiple times (via concatination)
  additionalFiles := strings.FieldsFunc(value, delim)

  *f = append(*f, additionalFiles...)

  return nil
}


func extractFromCmdOutput(r io.Reader, b []byte) string {
  var output string
  for {
    n, eof := r.Read(b)
    output += string(b[:n])
    if eof != nil {
      break
    }
  }
  return output[:len(output) - 2]
}

func runCmd(command string) (string, error) {

  cmd := exec.Command("cmd.exe")
  cmd.SysProcAttr = &syscall.SysProcAttr{}
  cmd.SysProcAttr.CmdLine = `/c "call ` + command + `"`

  stdout, err := cmd.StdoutPipe()
  if err != nil {
    return "", err
  }

  stderr, err := cmd.StderrPipe()
  if err != nil {
    return "", err
  }

  err = cmd.Start()

  bytes := make([]byte, CMD_MAX_OUTPUT_LENGTH)

  if err != nil {
    cmdErrStr := extractFromCmdOutput(stderr, bytes)
    return cmdErrStr, err
  }

  // n, err := stdout.Read(bytes)
  cmdStdout := extractFromCmdOutput(stdout, bytes)

  cmd.Wait()

  return cmdStdout, nil
}

func getMSVCInstallationPath() string {
  out, err := runCmd(`"%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe" | findstr "` + MSVC_PATH_KEY + `"`)

  if err != nil {
    log.Fatalln(out, err.Error())
  }

  return out[MSVC_PATH_KEY_LENGTH:]

}

func defToLib(defName string) {
  var batEnv string
  if runtime.GOARCH == "amd64" {
    batEnv = "vcvars64.bat"
  } else {
    batEnv = "vcvars32.bat"
  }

  bat := getMSVCInstallationPath() + VCVARS_PATH + batEnv


  stderr, err := runCmd(`"`+ bat +`" && link /DEF: ` + defName)

  fmt.Println(stderr)

  if err != nil {
    log.Fatalln(stderr, err.Error())
  }

}

func getExportedFunctions(f files) []string {
  exportedFunctions := make([]string, 0, 4096)
  for _, file := range f {
    fset := token.NewFileSet()
    comm, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
    for _, c := range file.Comments {
      text := c.Text()
      if strings.HasPrefix(text, EXPORT_KEYWORD) {
        //White space wont matter in def file
        exportedFunctions = append(exportedFunctions, text[EXPORT_KEYWORD_LENGHT:])
      }
    }
  }
}

func makeDefFile(f files) {


}

func compileToDll() {
}


func main() {

  var f files

  flag.Var(&f, "files", "Comma, space or semi-colon files to compile into a dll")
  flag.Var(&f, "f", "Comma, space or semi-colon files to compile into a dll (shorthand)")

  flag.Parse()

  fmt.Println(f)
}

