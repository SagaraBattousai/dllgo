
package main

import (
  "log"
  "fmt"
  "io"
  "os"
  "os/exec"
  "syscall"
  "runtime"
  "flag"
  "unicode"
  "strings"
  "go/parser"
  "go/token"
  "errors"
)

const (
  MSVC_PATH_KEY string = "installationPath"
  MSVC_PATH_KEY_LENGTH int = 18
  VCVARS_PATH string = `\VC\Auxiliary\Build\`
  CMD_MAX_OUTPUT_LENGTH int = 4096
  EXPORT_KEYWORD string = "export "
  EXPORT_KEYWORD_LENGHT int = 7
  DLL_EXTENSION string = ".dll"
  DEFAULT_DLL_NAME string = "go"
  DEF_EXTENSION string = ".def"
  DEF_SPACING string = "    " //Exactly 4 spaces (0x20 0x20 0x20 0x20)
)

type files []string
type outputName string

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

func (out *outputName) String() string {
  return string(*out)
}

func (out *outputName) Set(value string) error {
  var outValue string

  if string(*out) != "" {
    return errors.New("Output name is already set")
  }

  switch {
  case strings.HasSuffix(value, DLL_EXTENSION):
    outValue = value
  case value == "":
    outValue = DEFAULT_DLL_NAME + DLL_EXTENSION
  default:
    outValue = value + DLL_EXTENSION
  }

  *out = outputName(outValue)
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

  if len(output) < 2 {
    return ""
  }
  return output[:len(output) - 2]
}

func runCmd(command string, silent bool) (string, error) {

  cmd := exec.Command("cmd.exe")
  cmd.SysProcAttr = &syscall.SysProcAttr{}
  var flags string = "/c "
  if silent {
    flags += "/q "
  }
  cmd.SysProcAttr.CmdLine = flags + `"call ` + command + `"`

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
  out, err := runCmd(`"%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe" | findstr "` + MSVC_PATH_KEY + `"`, false)

  if err != nil {
    log.Fatalln(out, err.Error())
  }

  return out[MSVC_PATH_KEY_LENGTH:]

}

func defToLib(libraryName string) {
  var batEnv string
  var machine string

  if runtime.GOARCH == "amd64" {
    batEnv = "vcvars64.bat"
    machine = "x64"
  } else {
    batEnv = "vcvars32.bat"
    machine = "x86"
  }

  bat := getMSVCInstallationPath() + VCVARS_PATH + batEnv

  cmdString := fmt.Sprintf("\"%s\" && lib /DEF:%s.def /OUT:%s.lib /Machine:%s", bat, libraryName, libraryName, machine)

  stderr, err := runCmd(cmdString, true)

  if err != nil {
    log.Fatalln(stderr, err.Error())
  }

}

func getExportedFunctions(f []string) []string {
  exportedFunctions := make([]string, 0, 4096)
  for _, file := range f {
    fset := token.NewFileSet()
    comm, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
    if err != nil {
      continue
    }
    for _, c := range comm.Comments {
      text := c.Text()
      if strings.HasPrefix(text, EXPORT_KEYWORD) {
        //Removal of white space leads to neater Def file
        functionName := strings.TrimSpace(text[EXPORT_KEYWORD_LENGHT:])
        exportedFunctions = append(exportedFunctions, functionName)
      }
    }
  }
  return exportedFunctions
}

func CreateLinkingFiles(outputName string, functions ...string) {

  libraryName := strings.TrimSuffix(outputName, DLL_EXTENSION)

  file, err := os.Create(libraryName + DEF_EXTENSION)
  if err != nil {
    file.Close()
    log.Fatalln(err)
  }

  _, err = file.WriteString("LIBRARY" + DEF_SPACING + libraryName + "\nEXPORTS\n")

  if err != nil {
    file.Close()
    log.Fatalln(err)
  }

  //Could set the ordinals but there is a reson why not! Revisit?
  for _, fun := range functions {
    _, err = file.WriteString(DEF_SPACING + fun + "\n")
    if err != nil {
      file.Close()
      log.Fatalln(err)
    }
  }

  file.Close()
  defToLib(libraryName)
}

func compileToDll(outputName string, fileNames ...string) {
  compileArgs := append([]string{"build","-buildmode=c-shared","-o", outputName}, fileNames...)
  compileCmd := exec.Command("go",  compileArgs...)


  err := compileCmd.Run()

  if err != nil {
    log.Fatalln(err)
  }
}


func main() {

  var f files
  var output outputName

  flag.Var(&f, "files", "Comma, space or semi-colon files to compile into a dll")
  flag.Var(&f, "f", "Comma, space or semi-colon files to compile into a dll (short hand)")

  flag.Var(&output, "output", "Output name for dll")
  flag.Var(&output, "out", "Output name for dll (short hand)")
  flag.Var(&output, "o", "Output name for dll (super short hand)")

  flag.Parse()

  fileNames := []string(f)
  outputName := string(output)

  functions := getExportedFunctions(fileNames)

  CreateLinkingFiles(outputName, functions...)
  compileToDll(outputName, fileNames...)
}

