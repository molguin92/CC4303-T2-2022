# Cliente Stop-and-Wait en Go para Tareas 2 y 3 de CC4303 (2022-1)

## Instrucciones

Para utilizar este cliente, usted deberá primero compilarlo utilizando el compilador de Go.
Se recomienda seguir las siguientes instrucciones _dentro_ de la máquina virtual para la Tarea 3.

1. Instale Go siguiendo las las instrucciones en [este enlace](https://go.dev/doc/install). En resumen (para Linux):
   1. Borre posibles instalaciones previas de Go: `sudo rm -rf /usr/local/go`.
   2. Descargue el compilador:

      ```bash
      $ cd ~/Downloads
      $ sudo apt install wget -y
      ...
      $ wget https://go.dev/dl/go1.18.3.linux-amd64.tar.gz
      ```

   3. Descomprima e instale el compilador: `sudo tar -C /usr/local -xzf ~/Downloads/go1.18.3.linux-amd64.tar.gz`
   4. Use el siguiente comando para que la consola pueda encontrar el comando `go`: `echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile`
      (copie el comando textualmente, y fíjese en las comillas simples alrededor de `'export PATH=$PATH:/usr/local/go/bin'`).
   5. Reinicie la máquina virtual.
   6. Verifique su instalación de Go mediante el siguiente comando:
   
      ``` bash
      $ go version
      go version go1.18.3 linux/amd64
      ```
2. Ya instalado `go`, puede fácilmente obtener y compilar el código en este repositorio.
   1. Para descargar el código: `go get github.com/molguin92/CC4303-T2-2022`