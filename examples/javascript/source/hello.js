const helloFactory = function ({ React }) {
  const { string, func } = React.PropTypes;

  return function Hello(props) {
    // React wants propTypes & defaultProps
    // to be static.
    Hello.propTypes = {
      word: string,
      mode: string,

      actions: React.PropTypes.shape({
        setWord: func.isRequired,
        setMode: func.isRequired,
      }),
    };

    return {
      props, // set props

      componentDidUpdate() {
        this.refs.wordInput.getDOMNode().focus();
      },

      render() {
        const { word, mode } = this.props;

        const { setMode, setWord } = this.props.actions;

        const styles = {
          displayMode: {
            display: mode === "display" ? "inline" : "none",
          },

          editMode: {
            display: mode === "edit" ? "inline" : "none",
          },
        };

        const onKeyUp = function (e) {
          if (e.key !== "Enter") return;

          setWord(e.target.value);
          setMode("display");
        };

        return (
          <p>
            Hello,&nbsp;
            <span style={styles.displayMode} onClick={() => setMode("edit")}>
              {word}!
            </span>
            <input
              ref="wordInput"
              style={styles.editMode}
              placeholder={word}
              onKeyUp={onKeyUp}
            />
          </p>
        );
      },
    };
  };
};

export default helloFactory;
